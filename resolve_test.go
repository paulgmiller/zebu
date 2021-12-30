package main

import (
	"io"
	"strings"
	"testing"
)

func TestMarshal(t *testing.T) {
	response := `{"AD":true,"CD":false,"RA":true,"RD":true,"TC":false,"Status":0,"Question":[{"name":"northbriton.eth.","type":16}],"Answer":[{"name":"northbriton.eth","type":16,"TTL":3600,"data":"\"dnslink=/ipns/kbk5pvu86iew17anx6rzap7rufquifgobmvp814lq8h89spw2gz6hydqi9iyecu3bqr14zepxwy484ybgww75zldynonjdq590p3lf\""},{"name":"northbriton.eth","type":16,"TTL":3600,"data":"\"contenthash=0xe5010170003e6b3531717a693575717535646c35397033756f6c3464373171706561707676746339386e617063646665686b617673717561753173767538786961387273\""},{"name":"northbriton.eth","type":16,"TTL":3600,"data":"\"a=0xCbd6073f486714E6641bf87c22A9CEc25aCf5804\""}]}`
	r := io.NopCloser(strings.NewReader(response))
	hash, err := parsEthLink(r)
	if err != nil {
		t.Fatalf("can't parse %s", err)
	}
	if hash != "0xe5010170003e6b3531717a693575717535646c35397033756f6c3464373171706561707676746339386e617063646665686b617673717561753173767538786961387273" {
		t.Fatalf("got %s", hash)
	}

}
