package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	bolt "go.etcd.io/bbolt"
)

const bucketName = "samples"

type sample struct {
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
	Source    string  `json:"source"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	cmd := os.Args[1]

	dbPath := flag.String("db", getEnv("DB_PATH", "data.db"), "path to database")
	flag.CommandLine.Parse(os.Args[2:])

	fmt.Printf("Try to open: %s    ----> ", *dbPath)

	db, err := bolt.Open(*dbPath, 0600, nil)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	fmt.Print("open\n")
	defer db.Close()

	switch cmd {
	case "rename":
		if flag.NArg() != 2 {
			usage()
			os.Exit(1)
		}
		from, to := flag.Arg(0), flag.Arg(1)
		if err := renameSource(db, from, to); err != nil {
			log.Fatalf("rename: %v", err)
		}
	case "delete":
		if flag.NArg() != 1 {
			usage()
			os.Exit(1)
		}
		name := flag.Arg(0)
		if err := deleteSource(db, name); err != nil {
			log.Fatalf("delete: %v", err)
		}
	case "merge":
		if flag.NArg() != 2 {
			usage()
			os.Exit(1)
		}
		from, to := flag.Arg(0), flag.Arg(1)
		if err := mergeSource(db, from, to); err != nil {
			log.Fatalf("merge: %v", err)
		}
	case "list":
		if flag.NArg() != 0 {
			usage()
			os.Exit(1)
		}
		if err := listSources(db); err != nil {
			log.Fatalf("list: %v", err)
		}
	default:
		usage()
		os.Exit(1)
	}
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

func listSources(db *bolt.DB) error {
	return db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return fmt.Errorf("bucket %s not found", bucketName)
		}
		return b.ForEach(func(k, v []byte) error {
			if v != nil {
				return nil
			}
			fmt.Println(string(k))
			return nil
		})
	})
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  dbtool [flags] rename <from> <to>")
	fmt.Fprintln(os.Stderr, "  dbtool [flags] delete <name>")
	fmt.Fprintln(os.Stderr, "  dbtool [flags] merge <from> <to>")
	fmt.Fprintln(os.Stderr, "  dbtool [flags] list")
	flag.PrintDefaults()
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
