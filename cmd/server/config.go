package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

const defaultAddress = "127.0.0.1:19081"

func resolveAddress(flagAddress string) (string, error) {
	if strings.TrimSpace(flagAddress) != "" {
		return validateAddress(flagAddress)
	}
	if portText := strings.TrimSpace(os.Getenv("PORT")); portText != "" {
		port, err := strconv.Atoi(portText)
		if err != nil || port < 1 || port > 65535 {
			return "", fmt.Errorf("PORT 必须是 1 到 65535 的端口号")
		}
		return fmt.Sprintf("127.0.0.1:%d", port), nil
	}
	return defaultAddress, nil
}

func validateAddress(address string) (string, error) {
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return "", fmt.Errorf("监听地址无效：%w", err)
	}
	if host == "" {
		return "", fmt.Errorf("监听地址必须明确指定主机")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("监听端口无效")
	}
	return net.JoinHostPort(host, strconv.Itoa(port)), nil
}
