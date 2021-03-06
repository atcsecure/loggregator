package syslogwriter_test

import (
	"crypto/tls"
	"doppler/sinks/syslogwriter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/url"
	"sync"
	"time"
)

const MAX_ATTEMPTS = 5000

var _ = Describe("TlsWriter", func() {

	var serverStoppedChan <-chan struct{}
	shutdownChan := make(chan struct{})

	BeforeSuite(func() {
		serverStoppedChan = startTLSSyslogServer(shutdownChan)
	})

	AfterSuite(func() {
		close(shutdownChan)
		<-serverStoppedChan
	})

	It("connects and writes", func() {
		ts := time.Now().UnixNano()
		outputUrl, _ := url.Parse("syslog-tls://localhost:9998")
		w, _ := syslogwriter.NewTlsWriter(outputUrl, "appId", true)
		defer w.Close()
		err := w.Connect()
		Expect(err).ToNot(HaveOccurred())
		_, err = w.Write(14, []byte("just a test"), "test", "", ts)
		Expect(err).ToNot(HaveOccurred())
	})

	It("rejects self-signed certs", func() {
		outputUrl, _ := url.Parse("syslog-tls://localhost:9998")
		w, _ := syslogwriter.NewTlsWriter(outputUrl, "appId", false)
		defer w.Close()
		err := w.Connect()
		Expect(err).To(HaveOccurred())
	})

	It("returns an error for syslog scheme", func() {
		outputUrl, _ := url.Parse("syslog://localhost")
		_, err := syslogwriter.NewTlsWriter(outputUrl, "appId", false)
		Expect(err).To(HaveOccurred())
	})

	It("returns an error for https scheme", func() {
		outputUrl, _ := url.Parse("https://localhost")
		_, err := syslogwriter.NewTlsWriter(outputUrl, "appId", false)
		Expect(err).To(HaveOccurred())
	})
})

func startTLSSyslogServer(shutdownChan <-chan struct{}) <-chan struct{} {
	doneChan := make(chan struct{})
	cert, err := tls.X509KeyPair(localhostCert, localhostKey)
	if err != nil {
		panic(err)
	}
	config := &tls.Config{
		InsecureSkipVerify:     true,
		Certificates:           []tls.Certificate{cert},
		SessionTicketsDisabled: true,
	}
	listener, err := tls.Listen("tcp", "localhost:9998", config)
	if err != nil {
		panic(err)
	}

	var listenerStopped sync.WaitGroup
	listenerStopped.Add(1)

	go func() {
		<-shutdownChan
		listener.Close()
		listenerStopped.Wait()
		close(doneChan)
	}()

	go func() {
		var connectionsDone sync.WaitGroup

		defer listenerStopped.Done()
		defer connectionsDone.Wait()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			connectionsDone.Add(1)
			go func() {
				defer connectionsDone.Done()
				defer conn.Close()
				buffer := make([]byte, 1024)
				conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
				conn.Read(buffer)
			}()
		}
	}()

	for attempts := 0; attempts < MAX_ATTEMPTS; attempts++ {
		testConn, err := tls.Dial("tcp", "localhost:9998", config)
		if err == nil {
			testConn.Close()
			break
		}
		<-time.After(10 * time.Millisecond)
		if attempts == MAX_ATTEMPTS-1 {
			panic("TLS Server didn't start up!")
		}
	}
	return doneChan
}

var localhostCert = []byte(`-----BEGIN CERTIFICATE-----
MIIBdzCCASOgAwIBAgIBADALBgkqhkiG9w0BAQUwEjEQMA4GA1UEChMHQWNtZSBD
bzAeFw03MDAxMDEwMDAwMDBaFw00OTEyMzEyMzU5NTlaMBIxEDAOBgNVBAoTB0Fj
bWUgQ28wWjALBgkqhkiG9w0BAQEDSwAwSAJBAN55NcYKZeInyTuhcCwFMhDHCmwa
IUSdtXdcbItRB/yfXGBhiex00IaLXQnSU+QZPRZWYqeTEbFSgihqi1PUDy8CAwEA
AaNoMGYwDgYDVR0PAQH/BAQDAgCkMBMGA1UdJQQMMAoGCCsGAQUFBwMBMA8GA1Ud
EwEB/wQFMAMBAf8wLgYDVR0RBCcwJYILZXhhbXBsZS5jb22HBH8AAAGHEAAAAAAA
AAAAAAAAAAAAAAEwCwYJKoZIhvcNAQEFA0EAAoQn/ytgqpiLcZu9XKbCJsJcvkgk
Se6AbGXgSlq+ZCEVo0qIwSgeBqmsJxUu7NCSOwVJLYNEBO2DtIxoYVk+MA==
-----END CERTIFICATE-----`)

// localhostKey is the private key for localhostCert.
var localhostKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIBPAIBAAJBAN55NcYKZeInyTuhcCwFMhDHCmwaIUSdtXdcbItRB/yfXGBhiex0
0IaLXQnSU+QZPRZWYqeTEbFSgihqi1PUDy8CAwEAAQJBAQdUx66rfh8sYsgfdcvV
NoafYpnEcB5s4m/vSVe6SU7dCK6eYec9f9wpT353ljhDUHq3EbmE4foNzJngh35d
AekCIQDhRQG5Li0Wj8TM4obOnnXUXf1jRv0UkzE9AHWLG5q3AwIhAPzSjpYUDjVW
MCUXgckTpKCuGwbJk7424Nb8bLzf3kllAiA5mUBgjfr/WtFSJdWcPQ4Zt9KTMNKD
EUO0ukpTwEIl6wIhAMbGqZK3zAAFdq8DD2jPx+UJXnh0rnOkZBzDtJ6/iN69AiEA
1Aq8MJgTaYsDQWyU/hDq5YkDJc9e9DSCvUIzqxQWMQE=
-----END RSA PRIVATE KEY-----`)
