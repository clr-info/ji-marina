// Copyright ©2016 The ji-marina Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

var (
	dir  = flag.String("dir", "", "directory of the git repository")
	ip   = flag.String("ip", "piscine.in2p3.fr", "new ip of git repo")
	freq = flag.Duration("freq", 30*time.Minute, "mirror update frequency")
)

func main() {
	flag.Parse()

	if *dir == "" || *dir == "." {
		wd, err := os.Getwd()
		if err != nil {
			log.Fatal(err)
		}
		*dir = wd
	}

	err := os.Chdir(*dir)
	if err != nil {
		log.Fatal(err)
	}

	loop()
}

func loop() {
	ticker := time.NewTicker(*freq)
	defer ticker.Stop()

	err := update()
	if err != nil {
		log.Fatal(err)
	}

	for {
		select {
		case <-ticker.C:
			err := update()
			if err != nil {
				log.Printf("error: %v\n", err)
			}
		}
	}
}

func update() error {
	var err error
	defer func() {
		r := recover()
		if r != nil {
			switch r := r.(type) {
			case error:
				err = r
			default:
				err = fmt.Errorf("%v", r)
			}
		}
	}()

	fmt.Printf("\n\n\n\n\n")
	log.Printf("=== update ===")
	run("git", "fetch", "--all", "-p")
	run("git", "checkout", "master")
	run("git", "pull", "origin", "master")
	maybe(func() { run("git", "branch", "-D", "lioran") })
	run("git", "checkout", "-b", "lioran", "origin/master")
	run("find", *dir, "-name", "*.md", "-type", "f", "-exec", "sed", "-i", "-e", "s|https://gitlab.in2p3.fr/|http://"+*ip+"/|g", "{}", "+")
	run("git", "add", "-A", ".")
	run("git", "commit", "-m", "all: migrate to "+*ip)
	run("git", "push", "-f", "ji", "lioran")
	run("git", "push", "--all", "ji")

	return err
}

func run(cmd string, args ...string) {
	bin := exec.Command(cmd, args...)
	bin.Stdout = os.Stdout
	bin.Stderr = os.Stderr
	fmt.Printf("# %s\n", strings.Join(bin.Args, " "))
	err := bin.Run()
	if err != nil {
		log.Panicf("error: %s: %v\n", strings.Join(bin.Args, " "), err)
	}
}

func maybe(f func()) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("recovered: %v\n", r)
		}
	}()
	f()
}
