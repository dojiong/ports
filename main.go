package main

import (
	"flag"
	"fmt"
	"github.com/golang/glog"
	"io"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"sync"
)

var config_path = flag.String("config", "/etc/ports", "config file")

type Redirect struct {
	LocalAddr  string
	RemoteAddr string
}

func getConfig() []Redirect {
	f, err := os.Open(*config_path)
	if err != nil {
		glog.Errorf("open config file fail: %v", err)
		os.Exit(-1)
	}
	defer f.Close()

	data, err := ioutil.ReadAll(f)
	if err != nil {
		glog.Errorf("read config file fial: %v", err)
		os.Exit(-1)
	}

	redirects := make([]Redirect, 0)
	for ln, line := range strings.Split(string(data), "\n") {
		if strings.Trim(line, " \t\r") == "" {
			continue
		}

		var local_addr, remote_addr string
		var local_port, remote_port string

		if n, err := fmt.Sscanf(line, "%s %s %s %s", &local_addr, &local_port, &remote_addr, &remote_port); err != nil {
			glog.Errorf("invalid config line(%d): `%s` (%r)", ln, line, err)
			os.Exit(-1)
		} else if n != 4 {
			glog.Errorf("invalid config line(%d): `%s`", ln, line)
			os.Exit(-1)
		}
		redirects = append(redirects, Redirect{LocalAddr: net.JoinHostPort(local_addr, local_port),
			RemoteAddr: net.JoinHostPort(remote_addr, remote_port)})
	}

	return redirects
}

func (r *Redirect) Run() {
	l, err := net.Listen("tcp", r.LocalAddr)
	if err != nil {
		glog.Errorf("listen %s fail: %v", r.LocalAddr, err)
		return
	}
	defer l.Close()
	glog.Infof("redirect start: %s -> %s", r.LocalAddr, r.RemoteAddr)

	for {
		if conn, err := l.Accept(); err != nil {
			glog.Errorf("accept(%s) fail: %v", r.LocalAddr, err)
			break
		} else {
			go r.doRedirect(conn)
		}
	}
}

func (r *Redirect) doRedirect(conn net.Conn) {
	defer conn.Close()

	remote, err := net.Dial("tcp", r.RemoteAddr)
	if err != nil {
		glog.Errorf("connect to remote(%s) fail: %v", r.RemoteAddr, err)
		return
	}
	go io.Copy(conn, remote)
	io.Copy(remote, conn)
}

func main() {
	flag.Set("logtostderr", "true")
	flag.Parse()
	wg := &sync.WaitGroup{}
	for _, redirect := range getConfig() {
		wg.Add(1)
		go func() {
			redirect.Run()
			wg.Done()
		}()
	}
	wg.Wait()
}
