package regist

import (
	"log"
	"net"
	"net/rpc"
	"strings"
	"time"
)

func RegisterServiceToETCD(ServiceTarget string, value string) {
	dir = strings.TrimRight(ServiceTarget, "/") + "/"

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"localhost:2379"},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		panic(err)
	}

	kv := clientv3.NewKV(client)
	lease := clientv3.NewLease(client)
	var curLeaseId clientv3.LeaseID = 0

	for {
		if curLeaseId == 0 {
			leaseResp, err := lease.Grant(context.TODO(), 10)
			if err != nil {
				panic(err)
			}

			key := ServiceTarget + fmt.Sprintf("%d", leaseResp.ID)
			if _, err := kv.Put(context.TODO(), key, value, clientv3.WithLease(leaseResp.ID)); err != nil {
				panic(err)
			}
			curLeaseId = leaseResp.ID
		} else {
			// 续约租约，如果租约已经过期将curLeaseId复位到0重新走创建租约的逻辑
			if _, err := lease.KeepAliveOnce(context.TODO(), curLeaseId); err == rpctypes.ErrLeaseNotFound {
				curLeaseId = 0
				continue
			}
		}
		time.Sleep(time.Duration(1) * time.Second)
	}
}

type HelloService struct{}

func (p *HelloService) Hello(request string, reply *string) error {
	*reply = "hello:" + request
	return nil
}

var serviceTarget = "Hello"
var port = ":1234"
var host = "remote_host" // 伪代码

func main() {
	rpc.RegisterName("HelloService", new(HelloService))

	listener, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatal("ListenTCP error:", err)
	}

	conn, err := listener.Accept()
	if err != nil {
		log.Fatal("Accept error:", err)
	}

	go RegisterServiceToETCD(serviceTarget, host+port)
	rpc.ServeConn(conn)
}
