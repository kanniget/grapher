package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	cmd := os.Args[1]

	addr := flag.String("addr", getEnv("SERVER_ADDR", "http://localhost:8080"), "server address")
	flag.CommandLine.Parse(os.Args[2:])

	switch cmd {
	case "rename":
		if flag.NArg() != 2 {
			usage()
			os.Exit(1)
		}
		from, to := flag.Arg(0), flag.Arg(1)
		if err := renameSource(*addr, from, to); err != nil {
			log.Fatalf("rename: %v", err)
		}
	case "delete":
		if flag.NArg() != 1 {
			usage()
			os.Exit(1)
		}
		name := flag.Arg(0)
		if err := deleteSource(*addr, name); err != nil {
			log.Fatalf("delete: %v", err)
		}
	case "merge":
		if flag.NArg() != 2 {
			usage()
			os.Exit(1)
		}
		from, to := flag.Arg(0), flag.Arg(1)
		if err := mergeSource(*addr, from, to); err != nil {
			log.Fatalf("merge: %v", err)
		}
	case "list":
		if flag.NArg() != 0 {
			usage()
			os.Exit(1)
		}
		if err := listSources(*addr); err != nil {
			log.Fatalf("list: %v", err)
		}
	default:
		usage()
		os.Exit(1)
	}
}

func postJSON(url string, payload interface{}) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error: %s", strings.TrimSpace(string(body)))
	}
	return nil
}

func renameSource(addr, from, to string) error {
	return postJSON(addr+"/api/db/rename", map[string]string{"from": from, "to": to})
}

func deleteSource(addr, name string) error {
	return postJSON(addr+"/api/db/delete", map[string]string{"name": name})
}

func mergeSource(addr, from, to string) error {
	return postJSON(addr+"/api/db/merge", map[string]string{"from": from, "to": to})
}

func listSources(addr string) error {
	resp, err := http.Get(addr + "/api/db/list")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error: %s", strings.TrimSpace(string(body)))
	}
	var list []string
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return err
	}
	for _, name := range list {
		fmt.Println(name)
	}
	return nil
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
