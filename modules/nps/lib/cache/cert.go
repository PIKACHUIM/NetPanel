package cache

import (
	"bytes"
	"container/list"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/djylb/nps/lib/common"
)

type certEntry struct {
	isFile     bool
	certFile   string
	keyFile    string
	certText   string
	keyText    string
	cert       *tls.Certificate
	expire     time.Time
	lastUsed   time.Time
	lastReload time.Time
}

type CertManager struct {
	mu          sync.Mutex
	cache       *Cache
	loadMutexes map[string]*sync.Mutex
	sslTimeout  time.Duration
	idleTimeout time.Duration
	stopCh      chan struct{}
	stopOnce    sync.Once
}

func NewCertManager(maxEntries int, sslTimeout, idleTimeout time.Duration) *CertManager {
	m := &CertManager{
		cache:       New(maxEntries),
		loadMutexes: make(map[string]*sync.Mutex),
		sslTimeout:  sslTimeout,
		idleTimeout: idleTimeout,
		stopCh:      make(chan struct{}),
	}
	if idleTimeout > 0 {
		go m.runEvict()
	}
	return m
}

func (m *CertManager) runEvict() {
	ticker := time.NewTicker(m.idleTimeout)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.evictIdle()
		case <-m.stopCh:
			return
		}
	}
}

func (m *CertManager) Stop() {
	m.stopOnce.Do(func() {
		m.mu.Lock()
		m.cache.Clear()
		m.loadMutexes = make(map[string]*sync.Mutex)
		m.mu.Unlock()
		close(m.stopCh)
	})
}

func parseExpire(cert *tls.Certificate) (time.Time, error) {
	if len(cert.Certificate) == 0 {
		return time.Time{}, errors.New("no x509 data")
	}
	x, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return time.Time{}, err
	}
	return x.NotAfter, nil
}

func (m *CertManager) getLoadMutex(key string) *sync.Mutex {
	m.mu.Lock()
	defer m.mu.Unlock()
	if lm, ok := m.loadMutexes[key]; ok {
		return lm
	}
	lm := &sync.Mutex{}
	m.loadMutexes[key] = lm
	return lm
}

func (m *CertManager) Get(certInput, keyInput, mode, hash string) (*tls.Certificate, error) {
	now := time.Now()
	var isFile bool
	switch mode {
	case "file":
		isFile = true
	case "text":
		isFile = false
	default:
		return nil, errors.New("cert must be 'file' or 'text'")
	}

	m.mu.Lock()
	elem, ok := m.cache.Get(hash)
	if ok {
		e := elem.(*certEntry)
		e.lastUsed = now

		timedOut := m.sslTimeout > 0 && e.isFile && now.Sub(e.lastReload) >= m.sslTimeout
		expired := now.After(e.expire)
		cached := e.cert
		certFile, keyFile := e.certFile, e.keyFile
		m.mu.Unlock()

		if timedOut || expired {
			lm := m.getLoadMutex(hash)
			lm.Lock()
			newCert, err := tls.LoadX509KeyPair(certFile, keyFile)
			newExpire, err2 := parseExpire(&newCert)

			m.mu.Lock()
			e.lastReload = now
			if err == nil && err2 == nil {
				if expired || !bytes.Equal(newCert.Certificate[0], cached.Certificate[0]) {
					e.cert = &newCert
					e.expire = newExpire
					cached = &newCert
				}
			}
			m.mu.Unlock()
			lm.Unlock()
		}
		return cached, nil
	}
	m.mu.Unlock()

	lm := m.getLoadMutex(hash)
	lm.Lock()
	defer lm.Unlock()

	m.mu.Lock()
	elem, ok = m.cache.Get(hash)
	if ok {
		e := elem.(*certEntry)
		e.lastUsed = now
		c := e.cert
		m.mu.Unlock()
		return c, nil
	}
	m.mu.Unlock()

	var cert tls.Certificate
	var err error
	if isFile {
		certPath := common.GetPath(certInput)
		keyPath := common.GetPath(keyInput)
		if _, err1 := os.Stat(certPath); err1 != nil {
			return nil, errors.New("cert file not found")
		}
		if _, err2 := os.Stat(keyPath); err2 != nil {
			return nil, errors.New("key file not found")
		}
		cert, err = tls.LoadX509KeyPair(certPath, keyPath)
	} else {
		cert, err = tls.X509KeyPair([]byte(certInput), []byte(keyInput))
	}
	if err != nil {
		return nil, err
	}

	expire, err := parseExpire(&cert)
	if err != nil {
		return nil, err
	}

	entry := &certEntry{
		isFile:     isFile,
		certFile:   certInput,
		keyFile:    keyInput,
		certText:   certInput,
		keyText:    keyInput,
		cert:       &cert,
		expire:     expire,
		lastUsed:   now,
		lastReload: now,
	}

	m.mu.Lock()
	m.cache.Add(hash, entry)
	m.mu.Unlock()

	return &cert, nil
}

func (m *CertManager) evictIdle() {
	cutoff := time.Now().Add(-m.idleTimeout)
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cache.cache.Range(func(k, v interface{}) bool {
		key := k.(Key)
		elem := v.(*list.Element)
		e := elem.Value.(*entry).value.(*certEntry)
		if e.lastUsed.Before(cutoff) {
			m.cache.Remove(key)
			if hs, ok := key.(string); ok {
				delete(m.loadMutexes, hs)
			}
		}
		return true
	})
}
