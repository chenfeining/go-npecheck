package a

import (
	"context"
	"fmt"
	"net/http"
)

type MemberShip struct {
	a int
	c *float32
	B *ChildExample
	D ChildExample
}

type ChildExample struct {
	d int32
	C *GrandsonExample
}

type GrandsonExample struct {
	sd int32
	se *int32
}

type UserInfo struct {
	A *MemberShip
}

// 外部函数, slice 指针元素
func (o *Order) calcUserInfo(ctx context.Context, inputOrder *Order) {
	if inputOrder == nil {
		return
	}
	m := &MemberShip{}
	child := m.GetChildExample() // child may be nil pointer
	fmt.Println(child.C)         // want "potential nil pointer reference"

	temp2 := getSingleUserIndependent(ctx).A.GetChildExample().d
	//temp2 := getSingleUserIndependent(ctx).GetMemberShip().GetChildExample().d // 这个不是Selector
	//temp2 := o.getSingleUser(ctx).GetMemberShip().B
	// o 如果是内部变量，可以不检测，跳过， 但是其他的需要检测！
	fmt.Println(temp2)
	fmt.Println(inputOrder.orderId)

	if inputOrder.getSingleUser(ctx).GetMemberShip() != nil { // want "potential nil pointer reference"
		// inputOrder.getSingleUser(ctx) may be nil pointer
		temp2 := inputOrder.getSingleUser(ctx).GetMemberShip().B // want "potential nil pointer reference" "potential nil pointer reference"
		fmt.Println(temp2)
	}

	// 赋值语句
	temp := getSingleUserIndependent(ctx).A // 这个赋值语句也得校验
	fmt.Println(temp)

	// 表达式语句
	fmt.Println(o.getSingleUser(ctx).A)          // 判断叶子节点变量前面有一个fun(), 并且这个func 只返回一个结果，该结果为指针
	fmt.Println(getSingleUserIndependent(ctx).A) // 判断叶子节点变量前面有一个fun(), 并且这个func 只返回一个结果，该结果为指针

	for _, u := range o.getUserInfos(ctx) {
		if u != nil {
			fmt.Println(u.A)
		}
	}

	userInfoList := GetUserInfoList() // -- 后面再支持 GetPuFn().userInfo @eric.c
	switch len(userInfoList) {
	case 3:
		for _, u := range GetUserInfoList() {
			//if u != nil {
			// u may be a nil pointer
			fmt.Println(u.A) // want "potential nil pointer reference"
			//}
		}
	}
}

func CrossPtrAndVar(m *MemberShip) int {
	if m != nil {
		//d := m.D // 如果中间出现了非指针的变量, 目前是不能一直点 下去, 可以想办法处理一下 ---已解决
		fmt.Println(m.D.C)
	}

	return 0
}

func KTestSlicePtr(msList []*MemberShip) {
	for _, m := range msList {
		if m == nil {
			continue
		}
		fmt.Println(m.c)
	}
}

func (m *MemberShip) GetChildExample() *ChildExample {
	if m != nil {
		return m.B
	}

	return nil
}

func (u UserInfo) GetMemberShip() *MemberShip {
	//if u != nil {
	//	return u.A
	//}

	return nil
}

type Order struct {
	orderId int64
	orderNo string
}

func getSingleUserIndependent(ctx context.Context) *UserInfo {
	return nil
}

// 外部入参
func (o *Order) printInfo(userInfo *UserInfo) {
	if userInfo != nil {
		var tempA *MemberShip
		tempA = userInfo.A // tempA 是指针 99
		if tempA == nil {
			return
		}
		//if tempA != nil {
		fmt.Println(tempA.B)
		//}

		//if tempA.B != nil {
		if b := tempA.B; b != nil {
			fmt.Println(b.C) //
		}
	}
}

func (o *Order) getUserInfos(ctx context.Context) []*UserInfo {
	return nil
}

func (o *Order) getSingleUser(ctx context.Context) *UserInfo {
	return &UserInfo{}
}

// GetOrderDetailDep 多级嵌套测试， for-if-switch
// 同名变量
func (o *Order) GetOrderDetailDep(e *MemberShip) bool {
	if e != nil {
		return false
	}

	e.GetChildExample()
	fmt.Println(e.a) // 改动1 -- tech/eric.c/20230710/test_incr_check_01
	fmt.Println(e.a) // 改动2 -- tech/eric.c/20230710/test_incr_check_01

	fmt.Println(e.a) // 改动3 -- tech/eric.c/20230710/test_incr_check_02

	if o != nil {
		if o != nil {
			if e != nil {
				temp := e.a // 改动4 -- tech/eric.c/20230710/test_incr_check_02
				fmt.Println(temp)
			}
		}
	}

	if e != nil {
		fmt.Println(e.a) // 是否只检测到了增量的这一行
	}
	//	return false
	//}

	puResp, _ := GetPuFn()
	if puResp == nil {
		return false
	}
	for _, pu := range puResp.userInfo { // puResp 可能是空指针
		if pu != nil {
			fmt.Println(pu.A) // pu 也可能是空指针
		}
	}

	b := &BaoGetUserInfos{a: 1}
	userInfoList3 := GetUserInfoList()
	fmt.Println(e.a)
	for _, userInfo := range b.GetUserInfoList() {
		//if userInfo != nil {
		if userInfo == nil {
			continue
		}

		fmt.Println(userInfo.A) // For 循环语句里面，检测表达式、赋值语句、if 语句块 、switch
		c := &BaoGetUserInfos{a: 2}
		for _, userInfo2 := range c.GetUserInfoList() {
			if userInfo2 != nil {
				fmt.Println(userInfo2.A)
			}

			switch len(userInfoList3) {
			case 0: // 考虑case 里面的语句
				for _, u3 := range userInfoList3 {
					if u3 != nil {
						fmt.Println(u3.A)
					}
				}

				for _, u3 := range userInfoList3 {
					if u3 != nil {
						fmt.Println(u3.A)

						for _, u3 := range userInfoList3 {
							if u3 != nil {
								fmt.Println(u3.A)
							}
						}
					}
				}
			}
		}
	}
	return true
}

func GetFromHttpServ(input *UserInfo) error {
	if input != nil { // if 语句里面的变量，需要按顺序判断， 即if 语句里面的变量引用，也要判断指针到底有无check 过
		nodeC := input.A // if 语句块内赋值语句, 递归语句块
		if nodeC != nil {
			fmt.Println(nodeC.c)
		}

		//if input.A != nil {
		a := 1
		fmt.Println(a, 1, 2)
		resp, err := http.Get("http://httpbin.org/get") // 函数部分可能一直点下去，
		defer resp.Body.Close()

		if err != nil {
			fmt.Println(resp, err)
			return nil
		}
		//}

		return nil
	}

	//if input.A != nil { // @eric.c 后续优化
	//	fmt.Println(input.A.a)
	//}

	resp, err := http.Get("http://httpbin.org/get") // 函数部分可能一直点下去，
	defer resp.Body.Close()
	if err != nil {
		fmt.Println(resp, err)
		return nil
	}

	return nil
}

func GetUserInfoList() []*UserInfo {
	return nil
}

type PU struct {
	userInfo []*UserInfo
}

func GetPuFn() (*PU, error) {
	return nil, nil
}

func GetUserInfo() *UserInfo {
	return nil
}

type U struct {
	A int
}

func (u *U) GetUserInfo() *UserInfo {
	return nil
}

type BaoGetUserInfosInterface interface {
	GetUserInfoList() []*UserInfo
}

type BaoGetUserInfos struct {
	a int
}

func (b *BaoGetUserInfos) GetUserInfoList() []*UserInfo {
	return nil
}

func (b *BaoGetUserInfos) GetUserInfoSingle(ctx context.Context) *UserInfo {
	return nil
}

type BaoGetUserInfoInterface interface {
	GetUserInfo() *UserInfo
}

//
//// PrintMemberShip 普通函数
//func PrintMemberShip(e *MemberShip) {
//	if e != nil && e.B != nil && e.B.C != nil {
//		fmt.Println(e.B.C.sd)
//	}
//
//	//if e != nil {
//	fmt.Println(e.a)
//	//}
//}
//
//// GetOrderDetail 结构体方法
//func (o *Order) GetOrderDetail(e *MemberShip) bool {
//	//if e == nil {
//	//	return false
//	//}
//	//if e != nil {
//	fmt.Println(e.a)
//	//return false
//	//}
//
//	userInfo := GetUserInfoList()
//	if userInfo.A != nil && userInfo.A.B != nil { // && userInfo.A.B != nil {
//		fmt.Println(userInfo.A.B.C)
//		return false
//	}
//
//	return true
//}
