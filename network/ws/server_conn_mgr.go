/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/5/28 3:48 下午
 * @Desc: 连接管理器
 */

package ws

import (
	"github.com/dobyte/due/network"
	"sync"

	"github.com/gorilla/websocket"
)

type connMgr struct {
	rw     sync.RWMutex                    // 连接读写锁
	id     int64                           // 连接ID
	pool   sync.Pool                       // 连接池
	conns  map[*websocket.Conn]*serverConn // 连接集合
	server *server                         // 服务器
}

func newConnMgr(server *server) *connMgr {
	return &connMgr{
		server: server,
		conns:  make(map[*websocket.Conn]*serverConn),
		pool:   sync.Pool{New: func() interface{} { return &serverConn{} }},
	}
}

// 关闭连接
func (cm *connMgr) close() {
	cm.rw.Lock()
	defer cm.rw.RUnlock()

	for _, conn := range cm.conns {
		_ = conn.Close(false)
	}
}

// 分配连接
func (cm *connMgr) allocate(c *websocket.Conn) error {
	cm.rw.Lock()
	defer cm.rw.Unlock()

	if len(cm.conns) >= cm.server.opts.maxConnNum {
		return network.ErrTooManyConnection
	}

	cm.id++
	conn := cm.pool.Get().(*serverConn)
	conn.init(c, cm)
	cm.conns[c] = conn

	return nil
}

// 回收连接
func (cm *connMgr) recycle(conn *serverConn) {
	cm.rw.Lock()
	defer cm.rw.Unlock()

	delete(cm.conns, conn.conn)
	cm.pool.Put(conn)
}
