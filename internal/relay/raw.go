package relay

import (
	"net"
	"sync"
	"time"
)

func (r *Relay) handleTCPConn(c *net.TCPConn) error {
	rc, err := net.Dial("tcp", r.RemoteTCPAddr)
	if err != nil {
		return err
	}
	defer rc.Close()
	if err := rc.SetDeadline(time.Now().Add(TransportDeadLine)); err != nil {
		return err
	}
	if err := c.SetDeadline(time.Now().Add(TransportDeadLine)); err != nil {
		return err
	}
	transport(c, rc)
	return nil
}

func (r *Relay) handleOneUDPConn(addr string, ubc *udpBufferCh) {
	uaddr, _ := net.ResolveUDPAddr("udp", addr)
	rc, err := net.Dial("udp", r.RemoteUDPAddr)
	if err != nil {
		Logger.Info(err)
	}

	defer func() {
		rc.Close()
		close(ubc.Ch)
		delete(r.udpCache, addr)
	}()

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		buf := outboundBufferPool.Get().([]byte)
		for {
			i, err := rc.Read(buf)
			if err != nil {
				Logger.Info(err, 1)
				break
			}
			if err := r.keepAliveAndSetNextTimeout(rc); err != nil {
				Logger.Info(err)
				break
			}
			if _, err := r.UDPConn.WriteToUDP(buf[0:i], uaddr); err != nil {
				Logger.Info(err)
				break
			}
		}
		outboundBufferPool.Put(buf)
		wg.Done()
	}()

	for b := range ubc.Ch {
		if _, err := rc.Write(b); err != nil {
			Logger.Info(err)
			break
		}
		if err := r.keepAliveAndSetNextTimeout(rc); err != nil {
			Logger.Info(err)
			break
		}
	}
	wg.Wait()
}
