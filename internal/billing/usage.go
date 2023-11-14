package billing

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ellis-chen/fa/internal/storage"

	pb "github.com/ellis-chen/fa/third_party/billing/v2"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	// DOWNLOAD ...
	DOWNLOAD TransferType = "download"
	// CrossRegion ...
	CrossRegion TransferType = "cross-region"
)

var (
	bcOnce  sync.Once
	bcMutex sync.RWMutex
	c       Client

	_ Client = (*defaultClient)(nil)
	_ Client = (*noopClient)(nil)
)

type (

	// EfsUsage ...
	EfsUsage struct {
		UsageID      string
		Used         uint64
		TenantID     int64
		FileSystemID string
		CloudAccount string
		StartTime    time.Time
		EndTime      time.Time
	}

	// BackupUsage ...
	BackupUsage struct {
		UsageID   string
		Used      uint64
		TenantID  int64
		StartTime time.Time
		EndTime   time.Time
		Time      time.Time
	}

	// TransferType ...
	TransferType string

	// TransferUsage stands for a download usage
	TransferUsage struct {
		UsageID   string
		Size      int64
		TenantID  int64
		UserID    int
		Type      TransferType
		Name      string
		From      string
		To        string
		StartTime time.Time
		EndTime   time.Time
		ClientIP  string
	}

	Client interface {
		SendEfsUsage(ctx context.Context, v interface{})
		SendBackupUsage(ctx context.Context, v interface{})
		SendTransferUsage(ctx context.Context, v interface{})
		UpdateEfsUsage(usage int64)
		GetEfsUsage() int64
		Stop()
		pb.EventServiceClient
	}
)

func NewClient(addr string, storeMgr storage.StoreManager) Client {
	bcOnce.Do(func() {
		ReloadClient(addr, storeMgr)
	})

	return c
}

func ReloadClient(addr string, storeMgr storage.StoreManager) Client {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		logrus.Debugf("Connecting to fs-resource-usage at %s failed, fallback to noop", addr)
		c = &noopClient{}

		return c
	}

	logrus.Debugf("Connected to fs-resource-usage at %s", addr)
	bcMutex.Lock()
	c = &defaultClient{
		BillManager:        storeMgr,
		conn:               conn,
		EventServiceClient: pb.NewEventServiceClient(conn),
	}
	bcMutex.Unlock()

	return c
}

type defaultClient struct {
	conn *grpc.ClientConn
	pb.EventServiceClient
	storage.BillManager

	efsUsed atomic.Int64
}

// SendEfsUsage will send efs usage, if failed, retry up to 5 times or up to 10 seconds
func (bc *defaultClient) SendEfsUsage(ctx context.Context, v interface{}) {
	usage, ok := v.(EfsUsage)
	if !ok {
		return
	}

	var err error
	defer func() {
		if err != nil {
			logrus.Errorf("send usage failed, saving to db for further processing: %v", err)
			_ = bc.NewUsage(&storage.UsageItem{
				UsageID:      usage.UsageID,
				TenantID:     usage.TenantID,
				StartTime:    usage.StartTime,
				EndTime:      usage.EndTime,
				Used:         usage.Used,
				FileSystemID: usage.FileSystemID,
				CloudAccount: usage.CloudAccount,
				UsageType:    storage.UsageTypeEfs,
			})
		}
	}()

	if err = bc._sendEfsUsage(ctx, usage); err != nil {
		logrus.Errorf("Error sending efs-usage: %v", err)

		return
	}
	if err = bc.DeleteUsage(usage.UsageID); /*try delete usageItem if found any*/ err != nil {
		err = errors.Wrap(err, "err delete usage")

		return
	}
}

// _sendEfsUsage  ...
func (bc *defaultClient) _sendEfsUsage(ctx context.Context, usage EfsUsage) (err error) {
	select {
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), "err send efs-usage due to context done")
	default:
	}

	logrus.Debugf("efs-usage: [ %v ]", usage)
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	req := pb.PutEventRequest{
		ResourceType: pb.ResourceType_RESOURCE_TYPE_STORAGE_EFS,
		Efs: &pb.Efs{
			Time:         timestamppb.Now(),
			StartTime:    timestamppb.New(usage.StartTime),
			EndTime:      timestamppb.New(usage.EndTime),
			TenantId:     usage.TenantID,
			EfsId:        usage.FileSystemID,
			CloudAccount: usage.CloudAccount,
			Size:         int64(usage.Used),
			Unit:         "byte",
			Tags:         map[string]string{"id": usage.UsageID},
		},
	}
	_, err = c.PutEvent(ctx, &req)
	if err != nil {
		return errors.Wrap(err, "err PutEvent to fs-resource-usage")
	}

	return
}

// SendBackupUsage will send backup usage, if failed, retry up to 5 times or up to 10 seconds
func (bc *defaultClient) SendBackupUsage(ctx context.Context, v interface{}) {
	usage, ok := v.(BackupUsage)
	if !ok {
		logrus.Errorf(("not backup usage"))
	}

	var err error
	defer func() {
		if err != nil {
			logrus.Errorf("send usage failed, saving to db for further processing: %v", err)
			_ = bc.NewUsage(&storage.UsageItem{
				UsageID:   usage.UsageID,
				TenantID:  usage.TenantID,
				StartTime: usage.StartTime,
				EndTime:   usage.EndTime,
				Used:      usage.Used,
				UsageType: storage.UsageTypeEfs,
			})
		}
	}()

	if err = bc._sendBackupUsage(ctx, usage); err != nil {
		logrus.Errorf("Error sending efs-usage: %v", err)

		return
	}
	if err = bc.DeleteUsage(usage.UsageID); /*try delete usageItem if found any*/ err != nil {
		err = errors.Wrap(err, "err delete usage")

		return
	}
}

// _sendBackupUsage  ...
func (bc *defaultClient) _sendBackupUsage(ctx context.Context, usage BackupUsage) (err error) {
	select {
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), "err send backup usage due to context done")
	default:
	}

	logrus.Debugf("backup-usage: [ %v ]", usage)
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	req := pb.PutEventRequest{
		ResourceType: pb.ResourceType_RESOURCE_TYPE_BACKUP,
		Backup: &pb.Backup{
			Time:      timestamppb.Now(),
			StartTime: timestamppb.New(usage.StartTime),
			EndTime:   timestamppb.New(usage.EndTime),
			TenantId:  usage.TenantID,
			Size:      int64(usage.Used),
			Unit:      "byte",
			Tags:      map[string]string{"id": usage.UsageID},
		},
	}
	_, err = c.PutEvent(ctx, &req)
	if err != nil {
		return errors.Wrap(err, "err PutEvent to fs-resource-usage")
	}

	return
}

// SendTransferUsage will send backup usage, if failed, retry up to 5 times or up to 10 seconds
func (bc *defaultClient) SendTransferUsage(ctx context.Context, v interface{}) {
	usage, ok := v.(TransferUsage)
	if !ok {
		logrus.Errorf("not transfer type")
	}

	var err error
	defer func() {
		if err != nil {
			logrus.Errorf("send usage failed, saving to db for further processing: %v", err)
			_ = bc.NewUsage(&storage.UsageItem{
				UsageID:   usage.UsageID,
				TenantID:  usage.TenantID,
				StartTime: usage.StartTime,
				EndTime:   usage.EndTime,
				Used:      uint64(usage.Size),
				UserID:    usage.UserID,
				From:      usage.From,
				To:        usage.To,
				ClientIP:  usage.ClientIP,
				Name:      usage.Name,
				UsageType: storage.UsageTypeTransfer,
			})
		}
	}()

	if err = bc._sendTransferUsage(ctx, usage); err != nil {
		logrus.Errorf("Error sending efs-usage: %v", err)

		return
	}
	if err = bc.DeleteUsage(usage.UsageID); /*try delete usageItem if found any*/ err != nil {
		err = errors.Wrap(err, "err delete usage")

		return
	}
}

// _sendTransferUsage  ...
func (bc *defaultClient) _sendTransferUsage(ctx context.Context, usage TransferUsage) (err error) {
	select {
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), "err  sending transfer usage due to context done")
	default:
	}

	if usage.Size < 0 { // ignore the -1 condition
		return
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	logrus.Debugf("transfer : [ %v ]", usage)
	req := pb.PutEventRequest{
		ResourceType: pb.ResourceType_RESOURCE_TYPE_TRANSFER,
		Transfer: &pb.Transfer{
			TransferId: usage.UsageID,
			Time:       timestamppb.Now(),
			StartTime:  timestamppb.New(usage.StartTime),
			EndTime:    timestamppb.New(usage.EndTime),
			TenantId:   usage.TenantID,
			UserId:     int32(usage.UserID),
			Name:       usage.Name,
			From:       usage.From,
			To:         usage.To,
			Type:       string(usage.Type),
			Size:       usage.Size,
			Unit:       "byte",
			ClientIp:   usage.ClientIP,
		},
	}
	_, err = c.PutEvent(ctx, &req)
	if err != nil {
		return errors.Wrap(err, "err PutEvent to fs-resource-usage")
	}

	return
}

func (bc *defaultClient) UpdateEfsUsage(usage int64) {
	bc.efsUsed.Store(usage) //store
}

func (bc *defaultClient) GetEfsUsage() int64 {
	return bc.efsUsed.Load()
}

func (bc *defaultClient) Stop() {
	if bc.conn != nil {
		err := bc.conn.Close()
		if err != nil {
			logrus.Errorf("err close grpc conn: %v", err)
		}
	}
}

type noopClient struct {
	pb.EventServiceClient
}

func (e *noopClient) SendEfsUsage(ctx context.Context, v interface{}) {

}
func (e *noopClient) SendBackupUsage(ctx context.Context, v interface{}) {

}
func (e *noopClient) SendTransferUsage(ctx context.Context, v interface{}) {

}
func (e *noopClient) UpdateEfsUsage(usage int64) {

}
func (e *noopClient) GetEfsUsage() int64 {
	return 0
}
func (e *noopClient) Stop() {

}
