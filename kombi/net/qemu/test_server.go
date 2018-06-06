// Simple echo test server to test a qemu backed connection
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/kombi/net/qemu"
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

	lis, err := qemu.MakePipe()
	if err != nil {
		log.Fatalf("error opening pipe: %v", err)
	}
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
