// Package dnsp contains a simple DNS proxy.
package dnsp

import (
	"log"
	"regexp"

	"github.com/miekg/dns"
)

// Server implements a DNS server.
type Server struct {
	c *dns.Client
	s *dns.Server

	// White, when set to true, causes to server to work in white-listing mode.
	// It will only resolve queries that have been white-listed.
	//
	// When set to false, it will resolve anything that is not blacklisted.
	white bool

	// Hosts is a combined whitelist/blacklist. It contains both whitelist and blacklist entries.
	hosts hosts

	// Regex based whitelist and blacklist.
	rxWhitelist []regexp.Regexp
	rxBlacklist []regexp.Regexp
}

// NewServer creates a new Server with the given options.
func NewServer(o Options) (*Server, error) {
	s := Server{
		c: &dns.Client{},
		s: &dns.Server{
			Net:  o.Net,
			Addr: o.Bind,
		},
		white: o.Whitelist != "",
		hosts: hosts{},
	}
	if o.Whitelist != "" {
		if err := loadWhitelist(s.hosts, o.Whitelist); err != nil {
			return nil, err
		}
	}
	if o.Blacklist != "" {
		if err := loadBlacklist(s.hosts, o.Blacklist); err != nil {
			return nil, err
		}
	}
	s.s.Handler = dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		// If no upstream proxy is present, drop the query:
		if len(o.Resolve) == 0 {
			dns.HandleFailed(w, r)
			return
		}

		// Filter Questions:
		if r.Question = s.filter(r.Question); len(r.Question) == 0 {
			w.WriteMsg(r)
			return
		}

		// Proxy Query:
		for _, addr := range o.Resolve {
			in, rtt, err := s.c.Exchange(r, addr)
			if err != nil {
				log.Printf("dnsp: exchange failed: %s", err)
				continue
			}
			log.Printf("dnsp: exchange successful, rtt=%s", rtt) // debug
			w.WriteMsg(in)
			return
		}
		dns.HandleFailed(w, r)
	})
	return &s, nil
}

// ListenAndServe runs the server
func (s *Server) ListenAndServe() error {
	return s.s.ListenAndServe()
}

// Shutdown stops the server, closing any kernel buffers.
func (s *Server) Shutdown() error {
	return s.s.Shutdown()
}
