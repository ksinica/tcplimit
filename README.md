# tcplimit [![GoDoc](https://godoc.org/github.com/ksinica/tcplimit?status.svg)](https://godoc.org/github.com/ksinica/tcplimit)

This package provides basic traffic shaping for Go's stream-oriented network connections. It gives the ability to define global and per-connection (local) bandwidth limits.

## Usage

```go
limiter := tcplimit.NewLimiter()
// Set 1MB/s limit for cumulative traffic...
limiter.SetGlobalLimit(rate.Limit(1024 * 1024)) 
// ...but 50kB/s limit per one connection
limiter.SetLocalLimit(rate.Limit(10 * 1024))

// ...

limitedConn := limiter.LimitConn(conn)
go handleConnection(limitedConn)

// ...

// When we finally decided that we didn't want to cap global bandwidth 
// and give each active client a larger piece of traffic.
limiter.SetGlobalLimit(rate.Inf) 
limiter.SetLocalLimit(rate.Limit(100 * 1024)) // 100kB/s
```

## Testing

Although standard unit tests execute fast, it is advised to run also the "slow tests" (using `slow` build tag), which verify shaping constraints:

```
go test -v -race -tags slow .
```

## tcplimit-proxy
As a bonus, this package provides a `tcplmit-proxy` tool that is a basic HTTP proxy that can be used for traffic shaping.

![tcplimit-proxy](https://github.com/ksinica/tcplimit/assets/8190916/9e44883e-3841-46ff-9146-0220793bb705)

Proxy can be installed and started by invoking:
```
go install github.com/ksinica/tcplimit/cmd/tcplimit-proxy@latest
tcplimit-proxy --port 8080
```

Then it can be used for example with `wget` utility:
```
wget https://releases.ubuntu.com/22.04.3/ubuntu-22.04.3-desktop-amd64.iso -e use_proxy=yes -e https_proxy=127.0.0.1:8080
```

To change the global limit, one can do an HTTP PUT request on the `/limits/global` endpoint with a desired value as a payload:
```
curl -X PUT --data "524288" http://localhost:8080/limits/global 
```

The local limit can be manipulated in a similar way:
```
curl -X PUT --data "51200" http://localhost:8080/limits/local 
```

## License

Source code is available under the MIT [License](/LICENSE).
