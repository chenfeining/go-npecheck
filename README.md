# npecheck

[![pkg.go.dev][gopkg-badge]][gopkg]

`npecheck` Check for potential nil pointer reference exceptions to compensate for go linter's shortcomings in this area. This linter is supposed to be the most rigorous and complete NPE solution.

## How to use
```
$ go install github.com/chenfeining/go-npecheck/cmd/npecheck@latest
$ npecheck ./...
```

## Test case
The full use case can be found at testdata. Some examples are posted here

1. `npecheck` Function parameter is pointer, and its variable is directly referenced without validation
```go
func np1Example(d *DataInfo) {
    fmt.Println(d.A) // want "potential nil pointer reference"

    // d may be is a nil pointer, you had better to check it before reference.
    // such as :
    // if d != nil {
    //	  fmt.Println(d.A)
    // }
    
    // Or:
    // if d == nil {
    //	  return
    // }
    // fmt.Println(d.A)
    
    // Otherwise it is potential nil pointer reference, sometimes it's unexpected disaster
}

```

2. `npecheck` Function parameter is pointer, and its variables are directly referenced in a chain without validation
```go
func np2Example(d *DataInfo) {
	fmt.Println(d.A.B) // want "potential nil pointer reference" "potential nil pointer reference"

	// d is a potential nil pointer
	// d.A is also a potential nil pointer
	// You can follow the writing below, and will be more safe:

	// if d != nil && d.A != nil {
	//     fmt.Println(d.A.B)
	//}

	// Or:
	// if d == nil {
	//	 return
	// }
	//
	// if d.A == nil {
	//	 return
	// }
	//
	// fmt.Println(d.A.B)
}
```

3. `npecheck` A pointer variable obtained by calling an external function, unverified, directly referenced
```go
func np3Example() {
	d := GetDataInfo() // d is a pointer obtained by calling an external function
	fmt.Println(d.A)   // want "potential nil pointer reference"

	// d may be is a nil pointer, you had better to check it before reference.
	// such as :
	// if d != nil {
	//	fmt.Println(d.A)
	// }

	// Or:
	// if d == nil {
	//	return
	// }
	// fmt.Println(d.A)

	// Otherwise it is potential nil pointer reference, sometimes it's unexpected disaster
}

```

4. `npecheck` A pointer variable obtained by calling an external function, unverified, directly chain referenced
```go
func np4Example() {
	d := GetDataInfo()
	fmt.Println(d.A.B) // want "potential nil pointer reference" "potential nil pointer reference"

	// d is a potential nil pointer
	// d.A is also a potential nil pointer
	// You can follow the writing below, and will be more safe:

	// if d != nil && d.A != nil {
	//     fmt.Println(d.A.B)
	//}

	// Or:
	// if d == nil {
	//	 return
	// }
	//
	// if d.A == nil {
	//	 return
	// }
	//
	// fmt.Println(d.A.B)
}

```

5. `npecheck` Function input parameter is slice including pointer, and their elements are directly referenced without validation
```go
func np5Example(infoList []*Node) {
	for _, info := range infoList {
		fmt.Println(info.A) // want "potential nil pointer reference"
		// info is a potential nil pointer
		// It can be written as follows, and will be more safe.

		// if info != nil {
		// 	fmt.Println(info.A)
		// }

		// Or:
		// if info == nil {
		// 	  continue
		// }
		// fmt.Println(info.A)
	}
}

```

6. `npecheck` An slice including pointers obtained by a function, whose pointer elements are not checked and are directly referenced
```go
func np6Example() {
	infoList := GetDataInfoList()
	for _, info := range infoList {
		fmt.Println(info.A) // want "potential nil pointer reference"

		// info is a potential nil pointer
		// It can be written as follows, and will be more safe.

		// if info != nil {
		// 	fmt.Println(info.A)
		// }

		// Or:
		// if info == nil {
		// 	  continue
		// }
		// 	fmt.Println(info.A)
	}
}

```

7. `npecheck`  Function parameter is pointer, and its method is directly referenced without validation
```go
func np7Example1(d *DataInfo) {
	d.printDataInfo() // want "potential nil pointer reference"

	// d is a potential nil pointer
	// It can be written as follows, and will be more safe.
	// if d != nil {
	// 	d.printDataInfo()
	// }

	// Or:
	// if d == nil {
	// 	 return
	// }
	// d.printDataInfo()
}

```

8. `npecheck` Function parameter is pointer, and its method is directly referenced in chain without validation
```go
func np8Example(d *DataInfo) {
	_ = d.GetChildNodePtr().PrintScore() // want "potential nil pointer reference" "potential nil pointer reference"

	// d is a potential nil pointer reference
	// d.GetChildNodePtr() is also a potential nil pointer

	// It can be written as follows, and will be more safe.
	// if d != nil && d.GetChildNodePtr() != nil {
	// 	_ = d.GetChildNodePtr().PrintScore()
	// }

	// Or:
	// if d == nil {
	// 	 return
	// }
	//
	// if d.GetChildNodePtr() == nil {
	//	 return
	// }
	//
	// _ = d.GetChildNodePtr().PrintScore()
}

```

9. `npecheck` A pointer variable obtained by calling an external function, and its method is directly referenced in chain without validation

```go
func np9Example() {
	d := GetDataInfo()
	_ = d.GetChildNodePtr().PrintScore() // want "potential nil pointer reference" "potential nil pointer reference"

	// d is a potential nil pointer reference
	// d.GetChildNodePtr() is also a potential nil pointer

	// It can be written as follows, and will be more safe.
	// if d != nil && d.GetChildNodePtr() != nil {
	// 	_ = d.GetChildNodePtr().PrintScore()
	// }

	// Or:
	// if d == nil {
	// 	 return
	// }
	//
	// if d.GetChildNodePtr() == nil {
	//	 return
	// }
	//
	// _ = d.GetChildNodePtr().PrintScore()
}

```

10. `npecheck` Function parameter is pointer, and its child-node method is directly referenced in chain without validation
```go
func np10Example(d *DataInfo) {
	age := d.GetChildNodeNonPtr().GetGrandsonNodePtr().Age // want "potential nil pointer reference" "potential nil pointer reference"
	fmt.Println(age)
	// d is a potential nil pointer
	// d.GetChildNodeNonPtr() is not a pointer, just a struct variable
	// d.GetChildNodeNonPtr().GetGrandsonNodePtr() is a potential nil pointer

	// It can be written as follows, and will be more safe.
	// if d == nil {
	// 	 return
	// }
	//
	// if d.GetChildNodeNonPtr().GetGrandsonNodePtr() != nil {
	//	 age := d.GetChildNodeNonPtr().GetGrandsonNodePtr().Age
	//	 fmt.Println(age)
	// }

	// Or:
	// if d != nil && d.GetChildNodeNonPtr().GetGrandsonNodePtr() != nil {
	//	 age := d.GetChildNodeNonPtr().GetGrandsonNodePtr().Age
	//	 fmt.Println(age)
	// }
}

```

11. `npecheck` A pointer variable obtained by calling an external function, and its child-node method is directly referenced in chain without validation
```go
func np11Example() {
	d := GetDataInfo()
	age := d.GetChildNodeNonPtr().GetGrandsonNodePtr().Age // want "potential nil pointer reference" "potential nil pointer reference"
	fmt.Println(age)

	// d is a potential nil pointer
	// d.GetChildNodeNonPtr() is not a pointer, just a struct variable
	// d.GetChildNodeNonPtr().GetGrandsonNodePtr() is a potential nil pointer

	// It can be written as follows, and will be more safe.
	// if d == nil {
	// 	 return
	// }
	//
	// if d.GetChildNodeNonPtr().GetGrandsonNodePtr() != nil {
	//	 age := d.GetChildNodeNonPtr().GetGrandsonNodePtr().Age
	//	 fmt.Println(age)
	// }

	// Or:
	// if d != nil && d.GetChildNodeNonPtr().GetGrandsonNodePtr() != nil {
	//	 age := d.GetChildNodeNonPtr().GetGrandsonNodePtr().Age
	//	 fmt.Println(age)
	// }
}

```

12. `npecheck` Skip the parent node pointer check and directly verify the child node.
```go
func np12Example(d *DataInfo) {
	if d.A != nil { // want "potential nil pointer reference"
		fmt.Println(d.A.B) // want "potential nil pointer reference" "potential nil pointer reference"
	}

	// d is a potential nil pointer. It should valid d first.
	// It can be written as follows, and will be more safe.

	// if d != nil && d.A != nil {
	//	 fmt.Println(d.A.B)
	// }

	// Or:
	// if d == nil {
	//	 return
	// }
	//
	// if d.A != nil {
	//	 fmt.Println(d.A.B)
	//}

	// Or:
	// if d == nil {
	//	 return
	// }
	// if d.A == nil {
	//	 return
	// }
	// fmt.Println(d.A.B)
}

```


<!-- links -->
[gopkg]: https://github.com/chenfeining/go-npecheck/
[gopkg-badge]: https://pkg.go.dev/badge/github.com?status.svg
