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
		samples := readSamples(db)
		result := map[string]dataset{}
		for name, data := range samples {
			ds := dataset{Data: data}
			for _, src := range cfg.Sources {
				if sourceName(src) == name {
					ds.Units = src.Units
					ds.Type = src.Type
					break
				}
			}
			result[name] = ds
		}
		json.NewEncoder(w).Encode(result)
	})))

	// database maintenance endpoints
	http.Handle("/api/db/rename", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct{ From, To string }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.From == "" || req.To == "" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if err := renameSource(db, req.From, req.To); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})))

	http.Handle("/api/db/delete", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct{ Name string }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if err := deleteSource(db, req.Name); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})))

	http.Handle("/api/db/merge", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct{ From, To string }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.From == "" || req.To == "" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if err := mergeSource(db, req.From, req.To); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})))

	http.Handle("/api/db/list", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		list, err := listSources(db)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(list)
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

type dataset struct {
	Units string   `json:"units,omitempty"`
	Type  string   `json:"type,omitempty"`
	Data  []sample `json:"data"`
}

type pollSource struct {
	Name      string `json:"name"`
	Host      string `json:"host"`
	Community string `json:"community"`
	OID       string `json:"oid"`
	Units     string `json:"units,omitempty"`
	Type      string `json:"type,omitempty"`
	Version   string `json:"version,omitempty"`
}

type pollConfig struct {
	Sources   []pollSource `json:"sources"`
	Host      string       `json:"host"`      // legacy single source
	Community string       `json:"community"` // legacy single source
	OID       string       `json:"oid"`       // legacy single source
}

func sourceName(src pollSource) string {
	if src.Name != "" {
		return src.Name
	}
	return fmt.Sprintf("%s_%s", src.Host, strings.ReplaceAll(src.OID, ".", "-"))
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
		if c.Sources[i].Version == "" {
			c.Sources[i].Version = "1"
		}
	}

	return c
}

func poll(src pollSource, db *bolt.DB) {
	gs := &gosnmp.GoSNMP{
		Target:    src.Host,
		Port:      161,
		Community: src.Community,
		Version:   parseSNMPVersion(src.Version),
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

func renameSource(db *bolt.DB, from, to string) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return fmt.Errorf("bucket %s not found", bucketName)
		}
		src := b.Bucket([]byte(from))
		if src == nil {
			return fmt.Errorf("source %s not found", from)
		}
		dest, err := b.CreateBucket([]byte(to))
		if err != nil {
			return err
		}
		if err := src.ForEach(func(k, v []byte) error {
			var s sample
			if err := json.Unmarshal(v, &s); err == nil {
				s.Source = to
				if nb, err := json.Marshal(s); err == nil {
					v = nb
				}
			}
			return dest.Put(k, v)
		}); err != nil {
			return err
		}
		if err := b.DeleteBucket([]byte(from)); err != nil {
			return err
		}
		return nil
	})
}

func deleteSource(db *bolt.DB, name string) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return fmt.Errorf("bucket %s not found", bucketName)
		}
		return b.DeleteBucket([]byte(name))
	})
}

func mergeSource(db *bolt.DB, from, to string) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return fmt.Errorf("bucket %s not found", bucketName)
		}
		src := b.Bucket([]byte(from))
		if src == nil {
			return fmt.Errorf("source %s not found", from)
		}
		dest, err := b.CreateBucketIfNotExists([]byte(to))
		if err != nil {
			return err
		}
		if err := src.ForEach(func(k, v []byte) error {
			var s sample
			if err := json.Unmarshal(v, &s); err == nil {
				s.Source = to
				if nb, err := json.Marshal(s); err == nil {
					v = nb
				}
			}
			return dest.Put(k, v)
		}); err != nil {
			return err
		}
		return b.DeleteBucket([]byte(from))
	})
}

func listSources(db *bolt.DB) ([]string, error) {
	var names []string
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return fmt.Errorf("bucket %s not found", bucketName)
		}
		return b.ForEach(func(k, v []byte) error {
			if v != nil {
				return nil
			}
			names = append(names, string(k))
			return nil
		})
	})
	return names, err
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

func parseSNMPVersion(v string) gosnmp.SnmpVersion {
	switch strings.ToLower(v) {
	case "", "1", "v1", "version1":
		return gosnmp.Version1
	case "2", "2c", "v2c", "version2", "version2c":
		return gosnmp.Version2c
	case "3", "v3", "version3":
		return gosnmp.Version3
	default:
		log.Printf("unknown SNMP version %q, defaulting to v1", v)
		return gosnmp.Version1
	}
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
