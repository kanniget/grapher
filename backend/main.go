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
			for _, src := range cfg.Sources {
				poll(src, db)
			}
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
	Source    string  `json:"source"`
}

type pollSource struct {
	Name      string `json:"name"`
	Host      string `json:"host"`
	Community string `json:"community"`
	OID       string `json:"oid"`
}

type pollConfig struct {
	Sources   []pollSource `json:"sources"`
	Host      string       `json:"host"`      // legacy single source
	Community string       `json:"community"` // legacy single source
	OID       string       `json:"oid"`       // legacy single source
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

	if len(c.Sources) == 0 {
		// fall back to legacy single source definition
		src := pollSource{Host: c.Host, Community: c.Community, OID: c.OID}
		c.Sources = []pollSource{src}
	}

	for i := range c.Sources {
		if c.Sources[i].Host == "" {
			c.Sources[i].Host = "localhost"
		}
		if c.Sources[i].Community == "" {
			c.Sources[i].Community = "public"
		}
		if c.Sources[i].OID == "" {
			c.Sources[i].OID = ".1.3.6.1.2.1.1.3.0"
		}
	}

	return c
}

func poll(src pollSource, db *bolt.DB) {
	gs := &gosnmp.GoSNMP{
		Target:    src.Host,
		Port:      161,
		Community: src.Community,
		Version:   gosnmp.Version2c,
		Timeout:   time.Duration(2) * time.Second,
		Retries:   1,
	}
	if err := gs.Connect(); err != nil {
		log.Println("SNMP connect error:", err)
		return
	}
	defer gs.Conn.Close()
	pdu, err := gs.Get([]string{src.OID})
	if err != nil {
		log.Println("SNMP get error:", err)
		return
	}
	if len(pdu.Variables) == 0 {
		log.Println("No SNMP variables returned")
		return
	}
	val := float64(toInt(pdu.Variables[0].Value))
	name := src.Name
	if name == "" {
		name = fmt.Sprintf("%s_%s", src.Host, strings.ReplaceAll(src.OID, ".", "-"))
	}
	s := sample{Timestamp: time.Now().Unix(), Value: val, Source: name}
	db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		sb, err := b.CreateBucketIfNotExists([]byte(name))
		if err != nil {
			return err
		}
		key := []byte(fmt.Sprintf("%d", s.Timestamp))
		buf, _ := json.Marshal(s)
		return sb.Put(key, buf)
	})
	log.Printf("polled %s: %v", name, s)
}

func readSamples(db *bolt.DB) map[string][]sample {
	result := map[string][]sample{}
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return nil
		}
		b.ForEach(func(k, v []byte) error {
			if v != nil {
				return nil
			}
			sb := b.Bucket(k)
			if sb == nil {
				return nil
			}
			var list []sample
			sb.ForEach(func(kk, vv []byte) error {
				var s sample
				if err := json.Unmarshal(vv, &s); err == nil {
					list = append(list, s)
				}
				return nil
			})
			result[string(k)] = list
			return nil
		})
		return nil
	})
	return result
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
