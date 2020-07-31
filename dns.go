package main

import (
	"log"
	//	"sync"

	"github.com/miekg/dns"
)

// updateDNS updates the current slices of dns.RR so incoming requests get a
// fast answer
func updateDNS(s *dnsseeder) {

	var rr4std, rr4non, rr6std, rr6non []dns.RR

	s.mtx.RLock()

	// loop over each dns recprd type we need
	for t := range []int{dnsV4Std, dnsV4Non, dnsV6Std, dnsV6Non} {
		// FIXME above needs to be convertwd into one scan of theList if possible

		numRR := 0

		for _, nd := range s.theList {
			// when we reach max exit
			if numRR >= 25 {
				break
			}

			if nd.status != statusCG {
				continue
			}

			if t == dnsV4Std || t == dnsV4Non {
				if t == dnsV4Std && nd.dnsType == dnsV4Std {
					r := new(dns.A)
					r.Hdr = dns.RR_Header{Name: s.dnsHost + ".", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: s.ttl}
					r.A = nd.na.IP
					rr4std = append(rr4std, r)
					numRR++
				}

				// if the node is using a non standard port then add the encoded port info to DNS
				if t == dnsV4Non && nd.dnsType == dnsV4Non {
					r := new(dns.A)
					r.Hdr = dns.RR_Header{Name: "nonstd." + s.dnsHost + ".", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: s.ttl}
					r.A = nd.na.IP
					rr4non = append(rr4non, r)
					numRR++
					r = new(dns.A)
					r.Hdr = dns.RR_Header{Name: "nonstd." + s.dnsHost + ".", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: s.ttl}
					r.A = nd.nonstdIP
					rr4non = append(rr4non, r)
					numRR++
				}
			}
			if t == dnsV6Std || t == dnsV6Non {
				if t == dnsV6Std && nd.dnsType == dnsV6Std {
					r := new(dns.AAAA)
					r.Hdr = dns.RR_Header{Name: s.dnsHost + ".", Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: s.ttl}
					r.AAAA = nd.na.IP
					rr6std = append(rr6std, r)
					numRR++
				}
				// if the node is using a non standard port then add the encoded port info to DNS
				if t == dnsV6Non && nd.dnsType == dnsV6Non {
					r := new(dns.AAAA)
					r.Hdr = dns.RR_Header{Name: "nonstd." + s.dnsHost + ".", Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: s.ttl}
					r.AAAA = nd.na.IP
					rr6non = append(rr6non, r)
					numRR++
					r = new(dns.AAAA)
					r.Hdr = dns.RR_Header{Name: "nonstd." + s.dnsHost + ".", Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: s.ttl}
					r.AAAA = nd.nonstdIP
					rr6non = append(rr6non, r)
					numRR++
				}
			}

		}

	}

	s.mtx.RUnlock()

	config.dnsmtx.Lock()

	// update the map holding the details for this seeder
	for t := range []int{dnsV4Std, dnsV4Non, dnsV6Std, dnsV6Non} {
		switch t {
		case dnsV4Std:
			config.dns[s.dnsHost+".A"] = rr4std
		case dnsV4Non:
			config.dns["nonstd."+s.dnsHost+".A"] = rr4non
		case dnsV6Std:
			config.dns[s.dnsHost+".AAAA"] = rr6std
		case dnsV6Non:
			config.dns["nonstd."+s.dnsHost+".AAAA"] = rr6non
		}
	}

	config.dnsmtx.Unlock()

	if config.stats {
		s.counts.mtx.RLock()
		log.Printf("%s - DNS available: v4std: %v v4non: %v v6std: %v v6non: %v\n", s.name, len(rr4std), len(rr4non), len(rr6std), len(rr6non))
		log.Printf("%s - DNS counts: v4std: %v v4non: %v v6std: %v v6non: %v total: %v\n",
			s.name,
			s.counts.DNSCounts[dnsV4Std],
			s.counts.DNSCounts[dnsV4Non],
			s.counts.DNSCounts[dnsV6Std],
			s.counts.DNSCounts[dnsV6Non],
			s.counts.DNSCounts[dnsV4Std]+s.counts.DNSCounts[dnsV4Non]+s.counts.DNSCounts[dnsV6Std]+s.counts.DNSCounts[dnsV6Non])

		s.counts.mtx.RUnlock()

	}
}

// handleDNS processes a DNS request from remote client and returns
// a list of current ip addresses that the crawlers consider current.
func handleDNS(w dns.ResponseWriter, r *dns.Msg) {

	m := &dns.Msg{MsgHdr: dns.MsgHdr{
		Authoritative:      true,
		RecursionAvailable: false,
	}}
	m.SetReply(r)

	var qtype string

	switch r.Question[0].Qtype {
	case dns.TypeA:
		qtype = "A"
	case dns.TypeAAAA:
		qtype = "AAAA"
	case dns.TypeTXT:
		qtype = "TXT"
	case dns.TypeMX:
		qtype = "MX"
	case dns.TypeNS:
		qtype = "NS"
	default:
		qtype = "UNKNOWN"
	}

	config.dnsmtx.RLock()
	// if the dns map does not have a key for the request it will return an empty slice
	m.Answer = config.dns[r.Question[0].Name+qtype]
	config.dnsmtx.RUnlock()

	w.WriteMsg(m)

	if config.debug {
		log.Printf("debug - DNS response Type: standard  To IP: %s  Query Type: %s\n", w.RemoteAddr().String(), qtype)
	}
	// update the stats in a goroutine
	go updateDNSCounts(r.Question[0].Name, qtype)
}

// serve starts the requested DNS server listening on the requested port
func serve(net, port string) {
	server := &dns.Server{Addr: ":" + port, Net: net, TsigSecret: nil}
	if err := server.ListenAndServe(); err != nil {
		log.Printf("Failed to setup the "+net+" server: %v\n", err)
	}
}

/*

 */
