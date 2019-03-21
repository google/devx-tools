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

// adb_bin is a adb CLI compatible tool on top of H2O
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/google/waterfall/golang/client/adb"
	"google.golang.org/grpc"
)

func runCommand(ctx context.Context, args []string) error {
	parsedArgs, err := adb.ParseCommand(args)
	if err != nil {
		adb.Fallback(args)
	}

	if len(parsedArgs.Args) > 0 {
		if parsedArgs.Device == "" {
			// Let adb try to figure out if there is only one device.
			adb.Fallback(args)
		}

		// We dial outside gRPC domain in order to fallback to regular adb without grpc intervention
		cc, err := net.Dial("unix", "@h2o_"+parsedArgs.Device)
		if err != nil {
			adb.Fallback(args)
		}

		if fn, ok := adb.Commands[parsedArgs.Command]; ok {
			cfn := func() (*grpc.ClientConn, error) {
				return grpc.Dial(
					"",
					grpc.WithDialer(func(addr string, t time.Duration) (net.Conn, error) {
						return cc, nil
					}),
					grpc.WithBlock(),
					grpc.WithInsecure())
			}

			// Forward request uses a different service, so we need to dial accordingly.
			if parsedArgs.Command == "forward" {
				cfn = func() (*grpc.ClientConn, error) {
					return grpc.Dial(fmt.Sprintf("unix:@h2o_%s_xforward", parsedArgs.Device), grpc.WithInsecure())
				}
			}

			err := fn(ctx, cfn, parsedArgs.Args)
			if err != nil {
				if _, ok := err.(adb.ParseError); ok {
					adb.Fallback(args)
				}
			}
			return err
		}
		// Fallback for any command we don't recognize
		adb.Fallback(args)
	}
	// We don't support doing device lookups.
	adb.Fallback(args)

	// unreachable
	return nil
}

func main() {
	if err := runCommand(context.Background(), os.Args[1:]); err != nil {
		log.Fatalf("Failed to execute adb command: %v", err)
	}
}
