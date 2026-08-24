package main

import "testing"

func TestResolveAddressPriorityAndValidation(t *testing.T) {
	t.Setenv("PORT", "19444")
	address, err := resolveAddress("")
	if err != nil || address != "127.0.0.1:19444" {
		t.Fatalf("PORT 未解析为回环地址: %q, %v", address, err)
	}
	address, err = resolveAddress("127.0.0.1:19555")
	if err != nil || address != "127.0.0.1:19555" {
		t.Fatalf("-addr 未优先于 PORT: %q, %v", address, err)
	}
	if _, err := resolveAddress("127.0.0.1:0"); err == nil {
		t.Fatal("无效端口未被拒绝")
	}
}
