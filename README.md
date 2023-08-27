# wg-decoy

Decoy handshake for Wireguard. Allows some initial exchange between wireguard server and client in order to derail DPI filtering.

## Building

Run in the source directory:

```
make
```

Binary will be available in the `bin` directory.

Alternatively, you may invoke Go directly:

```
go build ./cmd/wg-decoy -o bin/wg-decoy
```

## Running on server (Linux only)

Add iptables rule to capture small packets targeted to wireguard port:

```sh
iptables -t mangle -A PREROUTING -i eth0 -p udp -m udp --dport 51820 -m addrtype --dst-type LOCAL -m length --length 0:44 -j TPROXY --on-port 1820 --on-ip 127.0.0.1
```

where instead of `eth0` use actual address of your public interface, instead of port `51820` use actual wireguard server port.

Then just run server:

```
wg-decoy server
```

## Running on client

Make sure you use some random but fixed port in your wireguard config like this:

```
ListenPort = 56218
```

and then right before connection start run command:

```
wg-decoy client SERVER_ADDRESS:SERVER_PORT LOCAL_PORT
```

where `SERVER_ADDRESS:SERVER_PORT` is a WG server endpoint and `LOCAL_PORT` is your `ListenPort` value in your client config.

If you use `wg-quick` it may be convenient to add wg-decoy invokation as a `PreUp` command in your client config.

## Synopsis

```
$ wg-decoy -h
Usage:

wg-decoy [OPTION]... server
wg-decoy [OPTION]... client <SERVER ADDRESS:PORT> <LOCAL WG PORT>
wg-decoy version

Options:
  -attempts uint
    	number of client request attempts (default 10)
  -bind-address string
    	server bind address (default "127.0.0.1:1820")
  -break-early
    	return as soon as minimal number of responses acquired (default true)
  -client-req string
    	client request (default "PING")
  -min-responses uint
    	minimal number of responses to collect (default 5)
  -server-resp string
    	server response (default "PONG")
  -timeout duration
    	network operation timeout (default 5s)
```
