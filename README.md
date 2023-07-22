# npecheck

[![pkg.go.dev][gopkg-badge]][gopkg]

`npecheck` finds code which returns nil even though it checks that error is not nil.

```go
func f() error {
	err := do()
	if err != nil {
		return nil // miss
	}
}
```

`npecheck` also finds code which returns error even though it checks that error is nil.

```go
func f() error {
	err := do()
	if err == nil {
		return err // miss
	}
}
```

`nilerr` ignores code which has a miss with ignore comment.

```go
func f() error {
	err := do()
	if err != nil {
		//lint:ignore nilerr reason
		return nil // ignore
	}
}
```

## How to use
```
$ go install github.com/chenfeining/go-npecheck/cmd/npecheck@latest
$ npecheck ./...
```

<!-- links -->
[gopkg]: https://github.com/chenfeining/go-npecheck/cmd/npecheck
[gopkg-badge]: https://pkg.go.dev/badge/github.com?status.svg
