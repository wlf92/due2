package node

import (
	"context"
	"github.com/dobyte/due/cluster"
	"github.com/dobyte/due/router"
	"github.com/dobyte/due/transport"
	innerclient "github.com/dobyte/due/transport/grpc/internal/client"
	"github.com/dobyte/due/transport/grpc/internal/code"
	"github.com/dobyte/due/transport/grpc/internal/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/status"
	"sync"
)

var clients sync.Map

type client struct {
	client pb.NodeClient
}

func NewClient(ep *router.Endpoint, opts *innerclient.Options) (*client, error) {
	cli, ok := clients.Load(ep.Address())
	if ok {
		return cli.(*client), nil
	}

	opts.Addr = ep.Address()
	opts.IsSecure = ep.IsSecure()

	conn, err := innerclient.Dial(opts)
	if err != nil {
		return nil, err
	}

	cc := &client{client: pb.NewNodeClient(conn)}
	clients.Store(ep.Address(), cc)

	return cc, nil
}

// Trigger 触发事件
func (c *client) Trigger(ctx context.Context, event cluster.Event, gid string, uid int64) (miss bool, err error) {
	_, err = c.client.Trigger(ctx, &pb.TriggerRequest{
		Event: int32(event),
		GID:   gid,
		UID:   uid,
	})

	miss = status.Code(err) == code.NotFoundSession

	return
}

// Deliver 投递消息
func (c *client) Deliver(ctx context.Context, gid, nid string, cid, uid int64, message *transport.Message) (miss bool, err error) {
	_, err = c.client.Deliver(ctx, &pb.DeliverRequest{
		GID: gid,
		NID: nid,
		CID: cid,
		UID: uid,
		Message: &pb.Message{
			Seq:    message.Seq,
			Route:  message.Route,
			Buffer: message.Buffer,
		},
	}, grpc.UseCompressor(gzip.Name))

	miss = status.Code(err) == code.NotFoundSession

	return
}
