package main

import "testing"

func TestParseConfigDefaultsToHighLoopbackPort(t *testing.T) {
	t.Setenv("PORT", "")
	configuration, err := parseConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if configuration.address != "127.0.0.1:19081" {
		t.Fatalf("默认监听地址错误: %s", configuration.address)
	}
}

func TestParseConfigUsesPortEnvironment(t *testing.T) {
	t.Setenv("PORT", "23456")
	configuration, err := parseConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if configuration.address != "127.0.0.1:23456" {
		t.Fatalf("PORT 监听地址错误: %s", configuration.address)
	}
}

func TestParseConfigRejectsInvalidPortEnvironment(t *testing.T) {
	t.Setenv("PORT", "not-a-port")
	if _, err := parseConfig(nil); err == nil {
		t.Fatal("非法 PORT 应返回错误")
	}
}
