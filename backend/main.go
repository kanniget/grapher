package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
	bolt "go.etcd.io/bbolt"
)

const bucketName = "samples"

func main() {
	cfgPath := getEnv("CONFIG_PATH", "config.json")
	cfg := loadPollConfig(cfgPath)
	snmpHost := cfg.Host
	snmpCommunity := cfg.Community
	snmpOID := cfg.OID
	pollInterval := getEnvDuration("POLL_INTERVAL", time.Minute)
	dbPath := getEnv("DB_PATH", "data.db")

	db, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		return err
	})

	go func() {
		for {
			poll(snmpHost, snmpCommunity, snmpOID, db)
			time.Sleep(pollInterval)
		}
	}()

	http.Handle("/api/data", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := readSamples(db)
		json.NewEncoder(w).Encode(data)
	})))

	http.Handle("/", http.FileServer(http.Dir("public")))

	addr := getEnv("ADDR", ":8080")
	log.Printf("Listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

type sample struct {
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
}

type pollConfig struct {
	Host      string `json:"host"`
	Community string `json:"community"`
	OID       string `json:"oid"`
}

func loadPollConfig(path string) pollConfig {
	b, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("read config: %v", err)
	}
	var c pollConfig
	if err := json.Unmarshal(b, &c); err != nil {
		log.Fatalf("parse config: %v", err)
	}
	if c.Host == "" {
		c.Host = "localhost"
	}
	if c.Community == "" {
		c.Community = "public"
	}
	if c.OID == "" {
		c.OID = ".1.3.6.1.2.1.1.3.0"
	}
	return c
}

func poll(host, community, oid string, db *bolt.DB) {
	gs := &gosnmp.GoSNMP{
		Target:    host,
		Port:      161,
		Community: community,
		Version:   gosnmp.Version2c,
		Timeout:   time.Duration(2) * time.Second,
		Retries:   1,
	}
	if err := gs.Connect(); err != nil {
		log.Println("SNMP connect error:", err)
		return
	}
	defer gs.Conn.Close()
	pdu, err := gs.Get([]string{oid})
	if err != nil {
		log.Println("SNMP get error:", err)
		return
	}
	if len(pdu.Variables) == 0 {
		log.Println("No SNMP variables returned")
		return
	}
	val := float64(toInt(pdu.Variables[0].Value))
	s := sample{Timestamp: time.Now().Unix(), Value: val}
	db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		key := []byte(fmt.Sprintf("%d", s.Timestamp))
		buf, _ := json.Marshal(s)
		return b.Put(key, buf)
	})
	log.Printf("polled: %v", s)
}

func readSamples(db *bolt.DB) []sample {
	samples := []sample{}
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		b.ForEach(func(k, v []byte) error {
			var s sample
			if err := json.Unmarshal(v, &s); err == nil {
				samples = append(samples, s)
			}
			return nil
		})
		return nil
	})
	return samples
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func toInt(v interface{}) int64 {
	switch t := v.(type) {
	case int:
		return int64(t)
	case uint:
		return int64(t)
	case int64:
		return t
	case uint64:
		return int64(t)
	case int32:
		return int64(t)
	case uint32:
		return int64(t)
	case float64:
		return int64(t)
	default:
		return 0
	}
}

// authMiddleware performs a simple OAuth2 token introspection if configured.
func authMiddleware(next http.Handler) http.Handler {
	introspectURL := os.Getenv("OAUTH2_INTROSPECT_URL")
	clientID := os.Getenv("OAUTH2_CLIENT_ID")
	clientSecret := os.Getenv("OAUTH2_CLIENT_SECRET")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if introspectURL == "" {
			next.ServeHTTP(w, r)
			return
		}
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		token := strings.TrimPrefix(auth, "Bearer ")
		form := url.Values{}
		form.Set("token", token)
		req, err := http.NewRequest("POST", introspectURL, strings.NewReader(form.Encode()))
		if err != nil {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if clientID != "" {
			req.SetBasicAuth(clientID, clientSecret)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, "auth error", http.StatusUnauthorized)
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var result struct {
			Active bool `json:"active"`
		}
		if err := json.Unmarshal(body, &result); err != nil || !result.Active {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
