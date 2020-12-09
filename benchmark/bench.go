package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net"
	"time"

	quic "github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/internal/testdata"
)

func main() {
	var bandwidth uint64
	var dataLen uint64
	flag.Uint64Var(&bandwidth, "b", 0, "bandwidth")
	flag.Uint64Var(&dataLen, "d", 1000000, "bandwidth")
	flag.Parse()

	var data []byte = make([]byte, dataLen)
	rand.Read(data)

	var ln quic.Listener
	serverAddr := make(chan net.Addr)
	handshakeChan := make(chan struct{})
	// start the server
	go func() {
		var err error
		tlsConf := testdata.GetTLSConfig()
		tlsConf.NextProtos = []string{"benchmark"}
		ln, err = quic.ListenAddr(
			"localhost:0",
			tlsConf,
			&quic.Config{SendBandwidth: bandwidth},
		)
		serverAddr <- ln.Addr()
		sess, err := ln.Accept(context.Background())
		if err != nil {
			panic(err)
		}
		// wait for the client to complete the handshake before sending the data
		// this should not be necessary, but due to timing issues on the CIs, this is necessary to avoid sending too many undecryptable packets
		<-handshakeChan
		str, err := sess.OpenStream()
		if err != nil {
			panic(err)
		}
		_, err = str.Write(data)
		if err != nil {
			panic(err)
		}
		err = str.Close()
		if err != nil {
			panic(err)
		}
	}()

	// start the client
	addr := <-serverAddr
	sess, err := quic.DialAddr(
		addr.String(),
		&tls.Config{InsecureSkipVerify: true, NextProtos: []string{"benchmark"}},
		&quic.Config{},
	)
	if err != nil {
		panic(err)
	}
	close(handshakeChan)
	str, err := sess.AcceptStream(context.Background())
	if err != nil {
		panic(err)
	}

	buf := &bytes.Buffer{}
	// measure the time it takes to download the dataLen bytes
	// note we're measuring the time for the transfer, i.e. excluding the handshake
	t0 := time.Now()
	_, err = io.Copy(buf, str)
	if err != nil {
		panic(err)
	}
	duration := time.Now().Sub(t0)

	fmt.Println("transfer rate [MB/s]", float64(dataLen)/1e6/duration.Seconds())

	ln.Close()
	sess.CloseWithError(0, "")
}
