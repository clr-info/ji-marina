// Copyright ©2016 The ji-marina Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"compress/gzip"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"
)

func main() {
	run()
}

func run() {
	addr := flag.String("addr", "192.168.1.3:8080", "address of the marina")
	flag.Parse()

	name := flag.Arg(0)

	start := time.Now()
	defer func() {
		log.Printf("pulling %q... [done] (%v)", name, time.Since(start))
	}()
	log.Printf("pulling %q...\n", name)

	resp, err := http.Get("http://" + *addr + "/docker-images/" + name)
	if err != nil {
		log.Fatalf("marina-get %q: %v\n", name, err)
	}
	defer resp.Body.Close()

	rz, err := gzip.NewReader(resp.Body)
	if err != nil {
		log.Fatalf("marina-get-gzip %q: %v\n", name, err)
	}
	defer rz.Close()

	load := exec.Command("docker", "load")
	load.Stdin = rz
	stdout, err := load.StdoutPipe()
	if err != nil {
		log.Fatalf("docker-load-stdout %q: %v\n", name, err)
	}

	stderr, err := load.StderrPipe()
	if err != nil {
		log.Fatalf("docker-load-stderr %q: %v\n", name, err)
	}

	err = load.Start()
	if err != nil {
		log.Fatalf("docker-load-start %q: %v\n", name, err)
		return
	}

	go io.Copy(os.Stdout, stdout)
	go io.Copy(os.Stderr, stderr)

	err = load.Wait()
	if err != nil {
		log.Fatalf("docker-load %q: %v\n", name, err)
	}
}
