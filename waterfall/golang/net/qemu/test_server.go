// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Simple echo test server to test a qemu backed connection
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/google/waterfall/golang/net/qemu"
	"golang.org/x/net/context"
	"golang.org/x/sync/errgroup"
)

var (
	conns   int
	recN    int
	logFile string
)

func init() {
	flag.IntVar(&conns, "conns", 1, "number of concurrent connections to accept")
	flag.IntVar(&recN, "rec_n", 4*1024*1024, "read rec_n before closing the connection")
	flag.StringVar(&logFile, "log_file", "", "write logs to file instead of stdout")
}

func main() {
	flag.Parse()

	if logFile != "" {
		f, err := os.Create(logFile)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		log.SetOutput(f)
	}

	pip, err := qemu.MakePipe("sockets/h2o")
	if err != nil {
		log.Fatalf("error opening pipe: %v", err)
	}
	lis := qemu.MakePipeConnBuilder(pip)
	defer lis.Close()

	eg, _ := errgroup.WithContext(context.Background())

	log.Printf("Runing %d conns \n", conns)
	for i := 0; i < conns; i++ {
		// Lets avoid variable aliasing
		func() {
			log.Println("Accepting connection...")
			c, err := lis.Accept()
			if err != nil {
				log.Fatalf("error accepting connection: %v", err)
			}
			log.Println("Accepted connection...")
			r, w := io.Pipe()
			eg.Go(func() error {
				defer w.Close()
				log.Println("Reading from conn...")
				n, err := io.Copy(w, c)
				if err != nil && err != io.EOF {
					return err
				}
				if n != int64(recN) {
					return fmt.Errorf("read %d bytes but was supposed to read %d", n, recN)
				}
				log.Println("Done reading from conn...")
				return nil
			})
			eg.Go(func() error {
				defer c.Close()
				log.Println("Writing to conn...")
				defer r.Close()
				n, err := io.Copy(c, r)
				if err != nil && err != io.EOF {
					return err
				}
				if n != int64(recN) {
					return fmt.Errorf("wrote %d bytes but was supposed to write %d", n, recN)
				}
				log.Println("Done Writing to conn...")
				return nil
			})
		}()
	}
	if err := eg.Wait(); err != nil {
		log.Fatalf("got error: %v", err)
	}
	log.Println("Dying...")
}
