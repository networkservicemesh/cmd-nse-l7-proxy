package dns

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/miekg/dns"
)

type ProxyRewriteServer struct {
	RewriteTO       net.IP
	ListenOn        string
	ResolveConfPath string
}

func (p *ProxyRewriteServer) ListenAndServe(ctx context.Context) <-chan error {
	var networks = []string{"tcp", "udp"}
	var result = make(chan error, len(networks))

	if p.RewriteTO == nil {
		result <- errors.New("RewriteTO is not set")
	}
	if p.ResolveConfPath == "" {
		p.ResolveConfPath = "/etc/resolv.conf"
	}
	for _, network := range networks {
		server := &dns.Server{Addr: p.ListenOn, Net: network}
		go func() {
			server.Handler = p
			defer server.Shutdown()
			select {
			case result <- server.ListenAndServe():
			case <-ctx.Done():
			}

		}()
	}

	return result
}

func (p *ProxyRewriteServer) ServeDNS(rw dns.ResponseWriter, m *dns.Msg) {
	config, err := dns.ClientConfigFromFile(p.ResolveConfPath)
	if err != nil {
		dns.HandleFailed(rw, m)
		return
	}
	var networks = []string{"tcp", "udp"}

	for _, network := range networks {
		var client = dns.Client{
			Net: network,
		}
		for _, addr := range config.Servers {
			var msg *dns.Msg
			fmt.Println(addr)
			fmt.Println(network)
			if msg, _, err = client.Exchange(m, fmt.Sprintf("%v:%v", addr, config.Port)); err != nil {
				fmt.Println(err.Error())
				continue
			}
			for _, answer := range msg.Answer {
				p.rewriteIP(answer)
			}
			if err := rw.WriteMsg(msg); err == nil {
				return
			}
		}
	}

	dns.HandleFailed(rw, m)

}
func (p *ProxyRewriteServer) rewriteIP(rr dns.RR) {
	switch rr.Header().Rrtype {
	case dns.TypeAAAA:
		if p.RewriteTO.To16() != nil {
			rr.(*dns.AAAA).AAAA = p.RewriteTO.To16()
		}
	case dns.TypeA:
		if p.RewriteTO.To4() != nil {
			rr.(*dns.A).A = p.RewriteTO.To4()
		}
	}
}
