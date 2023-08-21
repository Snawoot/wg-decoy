package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Snawoot/wg-decoy/client"
	"github.com/Snawoot/wg-decoy/server"
)

const (
	ProgName = "wg-decoy"
)

var (
	version = "undefined"

	timeout      = flag.Duration("timeout", 5*time.Second, "network operation timeout")
	bindAddress  = flag.String("bind-address", "127.0.0.1:1820", "server bind address")
	clientReq    = flag.String("client-req", "PING", "client request")
	serverResp   = flag.String("server-resp", "PONG", "server response")
	attempts     = flag.Uint("attempts", 10, "number of client request attempts")
	minResponses = flag.Uint("min-responses", 5, "minimal number of responses to collect")
	breakEarly   = flag.Bool("break-early", true, "return as soon as minimal number of responses acquired")
)

func usage() {
	out := flag.CommandLine.Output()
	fmt.Fprintln(out, "Usage:")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "%s server [OPTION]...\n", ProgName)
	fmt.Fprintf(out, "%s client <SERVER ADDRESS:PORT> <LOCAL WG PORT> [OPTION]...\n", ProgName)
	fmt.Fprintf(out, "%s version\n", ProgName)
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Options:")
	flag.PrintDefaults()
}

func cmdVersion() int {
	fmt.Println(version)
	return 0
}

func cmdClient(serverAddr string, localPort uint16) int {
	log.Printf("starting wg-decoy client. probing %s from local port %d\n", serverAddr, localPort)
	cfg := client.Config{
		RemoteAddress: serverAddr,
		LocalPort:     localPort,
		ClientReq:     []byte(*clientReq),
		ServerResp:    []byte(*serverResp),
	}

	appCtx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	dialCtx, cancel := context.WithTimeout(appCtx, *timeout)
	defer cancel()

	client, err := client.Dial(dialCtx, &cfg)
	if err != nil {
		log.Fatalf("client dial failed: %v", err)
	}
	defer client.Close()

	var successes uint
	for i := uint(0); i < *attempts && appCtx.Err() == nil; i++ {
		func() {
			log.Printf("doing exchange #%d...", i+1)
			reqCtx, cancel := context.WithTimeout(appCtx, *timeout)
			defer cancel()

			if err := client.Exchange(reqCtx); err != nil {
				log.Printf("exchange error: %v", err)
			} else {
				log.Print("successful exchange!")
				successes++
			}
		}()
		if *breakEarly && successes >= *minResponses {
			break
		}
	}
	if successes >= *minResponses {
		return 0
	}
	log.Printf("received insufficient number of successful responses: %d out of %d", successes, *minResponses)
	return 3
}

func cmdServer() int {
	log.Println("starting wg-decoy server")

	appCtx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	srvCfg := server.Config{
		BindAddress: *bindAddress,
		ClientReq:   []byte(*clientReq),
		ServerResp:  []byte(*serverResp),
	}
	_, err := server.New(appCtx, &srvCfg)
	if err != nil {
		log.Fatalf("can't start server: %v", err)
	}

	<-appCtx.Done()
	return 0
}

func run() int {
	flag.CommandLine.Usage = usage
	flag.Parse()
	args := flag.Args()

	switch len(args) {
	case 1:
		switch args[0] {
		case "server":
			return cmdServer()
		case "version":
			return cmdVersion()
		}
	case 3:
		switch args[0] {
		case "client":
			port, err := strconv.ParseUint(args[2], 10, 16)
			if err == nil {
				return cmdClient(args[1], uint16(port))
			}
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
	}
	usage()
	return 2
}

func main() {
	log.Default().SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
	log.Default().SetPrefix(strings.ToUpper(ProgName) + ": ")
	os.Exit(run())
}
