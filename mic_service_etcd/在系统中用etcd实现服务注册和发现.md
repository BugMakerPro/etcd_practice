## 在系统中用etcd实现服务注册和发现
> https://mp.weixin.qq.com/s/ZhJnjgQ2cOCrYn47MW2d8g

### 系统中实现服务注册与发现所需的基本功能有

- 服务注册：同一service的所有节点注册到相同目录下，节点启动后将自己的信息注册到所属服务的目录中。

- 健康检查：服务节点定时发送心跳，注册到服务目录中的信息设置一个较短的TTL，运行正常的服务节点每隔一段时间会去更新信息的TTL。

- 服务发现：通过名称能查询到服务提供外部访问的 IP 和端口号。比如网关代理服务时能够及时的发现服务中新增节点、丢弃不可用的服务节点，同时各个服务间也能感知对方的存在。

在分布式系统中，如何管理节点间的状态一直是一个难题，etcd 是由开发并维护的，它使用 Go 语言编写，并通过Raft 一致性算法处理日志复制以保证强一致性。etcd像是专门为集群环境的服务发现和注册而设计，它提供了数据 TTL 失效、数据改变监视、多值、目录监听、分布式锁原子操作等功能，可以方便的跟踪并管理集群节点的状态。

![Aaron Swartz](https://raw.githubusercontent.com/BugMakerPro/etcd_practice/main/mic_service_etcd/upload/mic.jpg)

我们写两个 Demo 程序，一个服务充当service，一个客户端程序充当网关代理。服务运行后会去etcd 以自己服务名命名的目录中注册服务节点，并定时续租（更新 TTL）。客户端从 etcd查询服务目录中的节点信息代理服务的请求，并且会在协程中实时监控服务目录中的变化，维护到自己的服务节点信息列表中。
```go

// 将服务注册到etcd上

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
```
```go

type HelloService struct {}

func (p *HelloService) Hello(request string, reply *string) error {
    *reply = "hello:" + request
    return nil
}

var serviceTarget = "Hello"
var port = ":1234"
var host = "remote_host"// 伪代码

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

    go RegisterServiceToETCD(serviceTarget,  host + port)
    rpc.ServeConn(conn)
}
```