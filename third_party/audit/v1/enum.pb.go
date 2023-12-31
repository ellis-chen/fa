// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.30.0
// 	protoc        (unknown)
// source: v1/enum.proto

package auditingv1

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// The enum type for the event name.
type EventName int32

const (
	EventName_EVENT_NAME_INVALID EventName = 0
	// 集群新建
	EventName_EVENT_NAME_CLUSTER_CREATED EventName = 1
	// 集群释放
	EventName_EVENT_NAME_CLUSTER_TERMINATED EventName = 2
	// 集群删除
	EventName_EVENT_NAME_CLUSTER_DELETED EventName = 3
	// 集群开机
	EventName_EVENT_NAME_CLUSTER_START EventName = 4
	// 集群关机
	EventName_EVENT_NAME_CLUSTER_STOP EventName = 5
	// 集群重命
	EventName_EVENT_NAME_CLUSTER_RENAME EventName = 6
	// 集群分区新建
	EventName_EVENT_NAME_CLUSTER_PARTITION_CREATED EventName = 7
	// 集群分区编辑
	EventName_EVENT_NAME_CLUSTER_PARTITION_UPDATED EventName = 8
	// 集群分区删除
	EventName_EVENT_NAME_CLUSTER_PARTITION_DELETED EventName = 9
	// 销毁分区
	EventName_EVENT_NAME_CLUSTER_PARTITION_TERMINATED EventName = 10
	// 集群节点新建
	EventName_EVENT_NAME_CLUSTER_NODE_CREATED EventName = 11
	// 集群节点编辑
	EventName_EVENT_NAME_CLUSTER_NODE_UPDATED EventName = 12
	// 集群节点释放
	EventName_EVENT_NAME_CLUSTER_NODE_TERMINATED EventName = 13
	// 节点节点开机
	EventName_EVENT_NAME_CLUSTER_NODE_START EventName = 14
	// 节点节点关机
	EventName_EVENT_NAME_CLUSTER_NODE_STOP EventName = 15
	// 集群节点远程访问SSH新建
	EventName_EVENT_NAME_CLUSTER_NODE_RA_SSH_CREATED EventName = 16
	// 集群节点远程访问SSH删除
	EventName_EVENT_NAME_CLUSTER_NODE_RA_SSH_DELETED EventName = 17
	// 集群节点远程访问VNC新建
	EventName_EVENT_NAME_CLUSTER_NODE_RA_VNC_CREATED EventName = 18
	// 集群节点远程访问VNC删除
	EventName_EVENT_NAME_CLUSTER_NODE_RA_VNC_DELETED EventName = 19
	// 集群节点远程访问RDP新建
	EventName_EVENT_NAME_CLUSTER_NODE_RA_RDP_CREATED EventName = 20
	// 集群节点远程访问RDP删除
	EventName_EVENT_NAME_CLUSTER_NODE_RA_RDP_DELETED EventName = 21
	// 全局挂载创建
	EventName_EVENT_NAME_CLUSTER_GLOBAL_MOUNT_CREATED EventName = 23
	// 全局挂载删除
	EventName_EVENT_NAME_CLUSTER_GLOBAL_MOUNT_DELETED EventName = 24
	// 新增分区挂载
	EventName_EVENT_NAME_CLUSTER_PARTITION_MOUNT_CREATED EventName = 22
	// 集群分区挂载删除
	EventName_EVENT_NAME_CLUSTER_PARTITION_MOUNT_DELETED EventName = 25
	// 转换分区
	EventName_EVENT_NAME_CLUSTER_MOUNT_TRANSFER EventName = 26
	// 集群安全组规则编辑
	EventName_EVENT_NAME_CLUSTER_SECURITY_GROUP_RULE_UPDATED EventName = 27
	// 集群autoscale添加节点
	EventName_EVENT_NAME_CLUSTER_AUTOSCALE_NODE_CREATED EventName = 28
	// 集群autoscale销毁节点
	EventName_EVENT_NAME_CLUSTER_AUTOSCALE_NODE_TERMINATED EventName = 29
	// 文件上传
	EventName_EVENT_NAME_STORAGE_FILE_UPLOADED EventName = 30
	// 文件下载
	EventName_EVENT_NAME_STORAGE_FILE_DOWNLOADED EventName = 31
	// 文件移动
	EventName_EVENT_NAME_STORAGE_FILE_MOVED EventName = 32
	// 文件拷贝
	EventName_EVENT_NAME_STORAGE_FILE_COPIED EventName = 33
	// 文件重命名
	EventName_EVENT_NAME_STORAGE_FILE_RENAMED EventName = 34
	// 文件删除
	EventName_EVENT_NAME_STORAGE_FILE_DELETED EventName = 35
	// 文件预览
	EventName_EVENT_NAME_STORAGE_FILE_PREVIEWD EventName = 36
	// 文件夹新建
	EventName_EVENT_NAME_STORAGE_FILE_CREATED EventName = 37
)

// Enum value maps for EventName.
var (
	EventName_name = map[int32]string{
		0:  "EVENT_NAME_INVALID",
		1:  "EVENT_NAME_CLUSTER_CREATED",
		2:  "EVENT_NAME_CLUSTER_TERMINATED",
		3:  "EVENT_NAME_CLUSTER_DELETED",
		4:  "EVENT_NAME_CLUSTER_START",
		5:  "EVENT_NAME_CLUSTER_STOP",
		6:  "EVENT_NAME_CLUSTER_RENAME",
		7:  "EVENT_NAME_CLUSTER_PARTITION_CREATED",
		8:  "EVENT_NAME_CLUSTER_PARTITION_UPDATED",
		9:  "EVENT_NAME_CLUSTER_PARTITION_DELETED",
		10: "EVENT_NAME_CLUSTER_PARTITION_TERMINATED",
		11: "EVENT_NAME_CLUSTER_NODE_CREATED",
		12: "EVENT_NAME_CLUSTER_NODE_UPDATED",
		13: "EVENT_NAME_CLUSTER_NODE_TERMINATED",
		14: "EVENT_NAME_CLUSTER_NODE_START",
		15: "EVENT_NAME_CLUSTER_NODE_STOP",
		16: "EVENT_NAME_CLUSTER_NODE_RA_SSH_CREATED",
		17: "EVENT_NAME_CLUSTER_NODE_RA_SSH_DELETED",
		18: "EVENT_NAME_CLUSTER_NODE_RA_VNC_CREATED",
		19: "EVENT_NAME_CLUSTER_NODE_RA_VNC_DELETED",
		20: "EVENT_NAME_CLUSTER_NODE_RA_RDP_CREATED",
		21: "EVENT_NAME_CLUSTER_NODE_RA_RDP_DELETED",
		23: "EVENT_NAME_CLUSTER_GLOBAL_MOUNT_CREATED",
		24: "EVENT_NAME_CLUSTER_GLOBAL_MOUNT_DELETED",
		22: "EVENT_NAME_CLUSTER_PARTITION_MOUNT_CREATED",
		25: "EVENT_NAME_CLUSTER_PARTITION_MOUNT_DELETED",
		26: "EVENT_NAME_CLUSTER_MOUNT_TRANSFER",
		27: "EVENT_NAME_CLUSTER_SECURITY_GROUP_RULE_UPDATED",
		28: "EVENT_NAME_CLUSTER_AUTOSCALE_NODE_CREATED",
		29: "EVENT_NAME_CLUSTER_AUTOSCALE_NODE_TERMINATED",
		30: "EVENT_NAME_STORAGE_FILE_UPLOADED",
		31: "EVENT_NAME_STORAGE_FILE_DOWNLOADED",
		32: "EVENT_NAME_STORAGE_FILE_MOVED",
		33: "EVENT_NAME_STORAGE_FILE_COPIED",
		34: "EVENT_NAME_STORAGE_FILE_RENAMED",
		35: "EVENT_NAME_STORAGE_FILE_DELETED",
		36: "EVENT_NAME_STORAGE_FILE_PREVIEWD",
		37: "EVENT_NAME_STORAGE_FILE_CREATED",
	}
	EventName_value = map[string]int32{
		"EVENT_NAME_INVALID":                             0,
		"EVENT_NAME_CLUSTER_CREATED":                     1,
		"EVENT_NAME_CLUSTER_TERMINATED":                  2,
		"EVENT_NAME_CLUSTER_DELETED":                     3,
		"EVENT_NAME_CLUSTER_START":                       4,
		"EVENT_NAME_CLUSTER_STOP":                        5,
		"EVENT_NAME_CLUSTER_RENAME":                      6,
		"EVENT_NAME_CLUSTER_PARTITION_CREATED":           7,
		"EVENT_NAME_CLUSTER_PARTITION_UPDATED":           8,
		"EVENT_NAME_CLUSTER_PARTITION_DELETED":           9,
		"EVENT_NAME_CLUSTER_PARTITION_TERMINATED":        10,
		"EVENT_NAME_CLUSTER_NODE_CREATED":                11,
		"EVENT_NAME_CLUSTER_NODE_UPDATED":                12,
		"EVENT_NAME_CLUSTER_NODE_TERMINATED":             13,
		"EVENT_NAME_CLUSTER_NODE_START":                  14,
		"EVENT_NAME_CLUSTER_NODE_STOP":                   15,
		"EVENT_NAME_CLUSTER_NODE_RA_SSH_CREATED":         16,
		"EVENT_NAME_CLUSTER_NODE_RA_SSH_DELETED":         17,
		"EVENT_NAME_CLUSTER_NODE_RA_VNC_CREATED":         18,
		"EVENT_NAME_CLUSTER_NODE_RA_VNC_DELETED":         19,
		"EVENT_NAME_CLUSTER_NODE_RA_RDP_CREATED":         20,
		"EVENT_NAME_CLUSTER_NODE_RA_RDP_DELETED":         21,
		"EVENT_NAME_CLUSTER_GLOBAL_MOUNT_CREATED":        23,
		"EVENT_NAME_CLUSTER_GLOBAL_MOUNT_DELETED":        24,
		"EVENT_NAME_CLUSTER_PARTITION_MOUNT_CREATED":     22,
		"EVENT_NAME_CLUSTER_PARTITION_MOUNT_DELETED":     25,
		"EVENT_NAME_CLUSTER_MOUNT_TRANSFER":              26,
		"EVENT_NAME_CLUSTER_SECURITY_GROUP_RULE_UPDATED": 27,
		"EVENT_NAME_CLUSTER_AUTOSCALE_NODE_CREATED":      28,
		"EVENT_NAME_CLUSTER_AUTOSCALE_NODE_TERMINATED":   29,
		"EVENT_NAME_STORAGE_FILE_UPLOADED":               30,
		"EVENT_NAME_STORAGE_FILE_DOWNLOADED":             31,
		"EVENT_NAME_STORAGE_FILE_MOVED":                  32,
		"EVENT_NAME_STORAGE_FILE_COPIED":                 33,
		"EVENT_NAME_STORAGE_FILE_RENAMED":                34,
		"EVENT_NAME_STORAGE_FILE_DELETED":                35,
		"EVENT_NAME_STORAGE_FILE_PREVIEWD":               36,
		"EVENT_NAME_STORAGE_FILE_CREATED":                37,
	}
)

func (x EventName) Enum() *EventName {
	p := new(EventName)
	*p = x
	return p
}

func (x EventName) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (EventName) Descriptor() protoreflect.EnumDescriptor {
	return file_v1_enum_proto_enumTypes[0].Descriptor()
}

func (EventName) Type() protoreflect.EnumType {
	return &file_v1_enum_proto_enumTypes[0]
}

func (x EventName) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use EventName.Descriptor instead.
func (EventName) EnumDescriptor() ([]byte, []int) {
	return file_v1_enum_proto_rawDescGZIP(), []int{0}
}

type EntityType int32

const (
	EntityType_ENTITY_TYPE_INVALID EntityType = 0
	EntityType_ENTITY_TYPE_CLUSTER EntityType = 1
	// for fa
	EntityType_ENTITY_TYPE_STORAGE EntityType = 2
)

// Enum value maps for EntityType.
var (
	EntityType_name = map[int32]string{
		0: "ENTITY_TYPE_INVALID",
		1: "ENTITY_TYPE_CLUSTER",
		2: "ENTITY_TYPE_STORAGE",
	}
	EntityType_value = map[string]int32{
		"ENTITY_TYPE_INVALID": 0,
		"ENTITY_TYPE_CLUSTER": 1,
		"ENTITY_TYPE_STORAGE": 2,
	}
)

func (x EntityType) Enum() *EntityType {
	p := new(EntityType)
	*p = x
	return p
}

func (x EntityType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (EntityType) Descriptor() protoreflect.EnumDescriptor {
	return file_v1_enum_proto_enumTypes[1].Descriptor()
}

func (EntityType) Type() protoreflect.EnumType {
	return &file_v1_enum_proto_enumTypes[1]
}

func (x EntityType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use EntityType.Descriptor instead.
func (EntityType) EnumDescriptor() ([]byte, []int) {
	return file_v1_enum_proto_rawDescGZIP(), []int{1}
}

var File_v1_enum_proto protoreflect.FileDescriptor

var file_v1_enum_proto_rawDesc = []byte{
	0x0a, 0x0d, 0x76, 0x31, 0x2f, 0x65, 0x6e, 0x75, 0x6d, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12,
	0x0b, 0x61, 0x75, 0x64, 0x69, 0x74, 0x69, 0x6e, 0x67, 0x2e, 0x76, 0x31, 0x2a, 0xea, 0x0b, 0x0a,
	0x09, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x16, 0x0a, 0x12, 0x45, 0x56,
	0x45, 0x4e, 0x54, 0x5f, 0x4e, 0x41, 0x4d, 0x45, 0x5f, 0x49, 0x4e, 0x56, 0x41, 0x4c, 0x49, 0x44,
	0x10, 0x00, 0x12, 0x1e, 0x0a, 0x1a, 0x45, 0x56, 0x45, 0x4e, 0x54, 0x5f, 0x4e, 0x41, 0x4d, 0x45,
	0x5f, 0x43, 0x4c, 0x55, 0x53, 0x54, 0x45, 0x52, 0x5f, 0x43, 0x52, 0x45, 0x41, 0x54, 0x45, 0x44,
	0x10, 0x01, 0x12, 0x21, 0x0a, 0x1d, 0x45, 0x56, 0x45, 0x4e, 0x54, 0x5f, 0x4e, 0x41, 0x4d, 0x45,
	0x5f, 0x43, 0x4c, 0x55, 0x53, 0x54, 0x45, 0x52, 0x5f, 0x54, 0x45, 0x52, 0x4d, 0x49, 0x4e, 0x41,
	0x54, 0x45, 0x44, 0x10, 0x02, 0x12, 0x1e, 0x0a, 0x1a, 0x45, 0x56, 0x45, 0x4e, 0x54, 0x5f, 0x4e,
	0x41, 0x4d, 0x45, 0x5f, 0x43, 0x4c, 0x55, 0x53, 0x54, 0x45, 0x52, 0x5f, 0x44, 0x45, 0x4c, 0x45,
	0x54, 0x45, 0x44, 0x10, 0x03, 0x12, 0x1c, 0x0a, 0x18, 0x45, 0x56, 0x45, 0x4e, 0x54, 0x5f, 0x4e,
	0x41, 0x4d, 0x45, 0x5f, 0x43, 0x4c, 0x55, 0x53, 0x54, 0x45, 0x52, 0x5f, 0x53, 0x54, 0x41, 0x52,
	0x54, 0x10, 0x04, 0x12, 0x1b, 0x0a, 0x17, 0x45, 0x56, 0x45, 0x4e, 0x54, 0x5f, 0x4e, 0x41, 0x4d,
	0x45, 0x5f, 0x43, 0x4c, 0x55, 0x53, 0x54, 0x45, 0x52, 0x5f, 0x53, 0x54, 0x4f, 0x50, 0x10, 0x05,
	0x12, 0x1d, 0x0a, 0x19, 0x45, 0x56, 0x45, 0x4e, 0x54, 0x5f, 0x4e, 0x41, 0x4d, 0x45, 0x5f, 0x43,
	0x4c, 0x55, 0x53, 0x54, 0x45, 0x52, 0x5f, 0x52, 0x45, 0x4e, 0x41, 0x4d, 0x45, 0x10, 0x06, 0x12,
	0x28, 0x0a, 0x24, 0x45, 0x56, 0x45, 0x4e, 0x54, 0x5f, 0x4e, 0x41, 0x4d, 0x45, 0x5f, 0x43, 0x4c,
	0x55, 0x53, 0x54, 0x45, 0x52, 0x5f, 0x50, 0x41, 0x52, 0x54, 0x49, 0x54, 0x49, 0x4f, 0x4e, 0x5f,
	0x43, 0x52, 0x45, 0x41, 0x54, 0x45, 0x44, 0x10, 0x07, 0x12, 0x28, 0x0a, 0x24, 0x45, 0x56, 0x45,
	0x4e, 0x54, 0x5f, 0x4e, 0x41, 0x4d, 0x45, 0x5f, 0x43, 0x4c, 0x55, 0x53, 0x54, 0x45, 0x52, 0x5f,
	0x50, 0x41, 0x52, 0x54, 0x49, 0x54, 0x49, 0x4f, 0x4e, 0x5f, 0x55, 0x50, 0x44, 0x41, 0x54, 0x45,
	0x44, 0x10, 0x08, 0x12, 0x28, 0x0a, 0x24, 0x45, 0x56, 0x45, 0x4e, 0x54, 0x5f, 0x4e, 0x41, 0x4d,
	0x45, 0x5f, 0x43, 0x4c, 0x55, 0x53, 0x54, 0x45, 0x52, 0x5f, 0x50, 0x41, 0x52, 0x54, 0x49, 0x54,
	0x49, 0x4f, 0x4e, 0x5f, 0x44, 0x45, 0x4c, 0x45, 0x54, 0x45, 0x44, 0x10, 0x09, 0x12, 0x2b, 0x0a,
	0x27, 0x45, 0x56, 0x45, 0x4e, 0x54, 0x5f, 0x4e, 0x41, 0x4d, 0x45, 0x5f, 0x43, 0x4c, 0x55, 0x53,
	0x54, 0x45, 0x52, 0x5f, 0x50, 0x41, 0x52, 0x54, 0x49, 0x54, 0x49, 0x4f, 0x4e, 0x5f, 0x54, 0x45,
	0x52, 0x4d, 0x49, 0x4e, 0x41, 0x54, 0x45, 0x44, 0x10, 0x0a, 0x12, 0x23, 0x0a, 0x1f, 0x45, 0x56,
	0x45, 0x4e, 0x54, 0x5f, 0x4e, 0x41, 0x4d, 0x45, 0x5f, 0x43, 0x4c, 0x55, 0x53, 0x54, 0x45, 0x52,
	0x5f, 0x4e, 0x4f, 0x44, 0x45, 0x5f, 0x43, 0x52, 0x45, 0x41, 0x54, 0x45, 0x44, 0x10, 0x0b, 0x12,
	0x23, 0x0a, 0x1f, 0x45, 0x56, 0x45, 0x4e, 0x54, 0x5f, 0x4e, 0x41, 0x4d, 0x45, 0x5f, 0x43, 0x4c,
	0x55, 0x53, 0x54, 0x45, 0x52, 0x5f, 0x4e, 0x4f, 0x44, 0x45, 0x5f, 0x55, 0x50, 0x44, 0x41, 0x54,
	0x45, 0x44, 0x10, 0x0c, 0x12, 0x26, 0x0a, 0x22, 0x45, 0x56, 0x45, 0x4e, 0x54, 0x5f, 0x4e, 0x41,
	0x4d, 0x45, 0x5f, 0x43, 0x4c, 0x55, 0x53, 0x54, 0x45, 0x52, 0x5f, 0x4e, 0x4f, 0x44, 0x45, 0x5f,
	0x54, 0x45, 0x52, 0x4d, 0x49, 0x4e, 0x41, 0x54, 0x45, 0x44, 0x10, 0x0d, 0x12, 0x21, 0x0a, 0x1d,
	0x45, 0x56, 0x45, 0x4e, 0x54, 0x5f, 0x4e, 0x41, 0x4d, 0x45, 0x5f, 0x43, 0x4c, 0x55, 0x53, 0x54,
	0x45, 0x52, 0x5f, 0x4e, 0x4f, 0x44, 0x45, 0x5f, 0x53, 0x54, 0x41, 0x52, 0x54, 0x10, 0x0e, 0x12,
	0x20, 0x0a, 0x1c, 0x45, 0x56, 0x45, 0x4e, 0x54, 0x5f, 0x4e, 0x41, 0x4d, 0x45, 0x5f, 0x43, 0x4c,
	0x55, 0x53, 0x54, 0x45, 0x52, 0x5f, 0x4e, 0x4f, 0x44, 0x45, 0x5f, 0x53, 0x54, 0x4f, 0x50, 0x10,
	0x0f, 0x12, 0x2a, 0x0a, 0x26, 0x45, 0x56, 0x45, 0x4e, 0x54, 0x5f, 0x4e, 0x41, 0x4d, 0x45, 0x5f,
	0x43, 0x4c, 0x55, 0x53, 0x54, 0x45, 0x52, 0x5f, 0x4e, 0x4f, 0x44, 0x45, 0x5f, 0x52, 0x41, 0x5f,
	0x53, 0x53, 0x48, 0x5f, 0x43, 0x52, 0x45, 0x41, 0x54, 0x45, 0x44, 0x10, 0x10, 0x12, 0x2a, 0x0a,
	0x26, 0x45, 0x56, 0x45, 0x4e, 0x54, 0x5f, 0x4e, 0x41, 0x4d, 0x45, 0x5f, 0x43, 0x4c, 0x55, 0x53,
	0x54, 0x45, 0x52, 0x5f, 0x4e, 0x4f, 0x44, 0x45, 0x5f, 0x52, 0x41, 0x5f, 0x53, 0x53, 0x48, 0x5f,
	0x44, 0x45, 0x4c, 0x45, 0x54, 0x45, 0x44, 0x10, 0x11, 0x12, 0x2a, 0x0a, 0x26, 0x45, 0x56, 0x45,
	0x4e, 0x54, 0x5f, 0x4e, 0x41, 0x4d, 0x45, 0x5f, 0x43, 0x4c, 0x55, 0x53, 0x54, 0x45, 0x52, 0x5f,
	0x4e, 0x4f, 0x44, 0x45, 0x5f, 0x52, 0x41, 0x5f, 0x56, 0x4e, 0x43, 0x5f, 0x43, 0x52, 0x45, 0x41,
	0x54, 0x45, 0x44, 0x10, 0x12, 0x12, 0x2a, 0x0a, 0x26, 0x45, 0x56, 0x45, 0x4e, 0x54, 0x5f, 0x4e,
	0x41, 0x4d, 0x45, 0x5f, 0x43, 0x4c, 0x55, 0x53, 0x54, 0x45, 0x52, 0x5f, 0x4e, 0x4f, 0x44, 0x45,
	0x5f, 0x52, 0x41, 0x5f, 0x56, 0x4e, 0x43, 0x5f, 0x44, 0x45, 0x4c, 0x45, 0x54, 0x45, 0x44, 0x10,
	0x13, 0x12, 0x2a, 0x0a, 0x26, 0x45, 0x56, 0x45, 0x4e, 0x54, 0x5f, 0x4e, 0x41, 0x4d, 0x45, 0x5f,
	0x43, 0x4c, 0x55, 0x53, 0x54, 0x45, 0x52, 0x5f, 0x4e, 0x4f, 0x44, 0x45, 0x5f, 0x52, 0x41, 0x5f,
	0x52, 0x44, 0x50, 0x5f, 0x43, 0x52, 0x45, 0x41, 0x54, 0x45, 0x44, 0x10, 0x14, 0x12, 0x2a, 0x0a,
	0x26, 0x45, 0x56, 0x45, 0x4e, 0x54, 0x5f, 0x4e, 0x41, 0x4d, 0x45, 0x5f, 0x43, 0x4c, 0x55, 0x53,
	0x54, 0x45, 0x52, 0x5f, 0x4e, 0x4f, 0x44, 0x45, 0x5f, 0x52, 0x41, 0x5f, 0x52, 0x44, 0x50, 0x5f,
	0x44, 0x45, 0x4c, 0x45, 0x54, 0x45, 0x44, 0x10, 0x15, 0x12, 0x2b, 0x0a, 0x27, 0x45, 0x56, 0x45,
	0x4e, 0x54, 0x5f, 0x4e, 0x41, 0x4d, 0x45, 0x5f, 0x43, 0x4c, 0x55, 0x53, 0x54, 0x45, 0x52, 0x5f,
	0x47, 0x4c, 0x4f, 0x42, 0x41, 0x4c, 0x5f, 0x4d, 0x4f, 0x55, 0x4e, 0x54, 0x5f, 0x43, 0x52, 0x45,
	0x41, 0x54, 0x45, 0x44, 0x10, 0x17, 0x12, 0x2b, 0x0a, 0x27, 0x45, 0x56, 0x45, 0x4e, 0x54, 0x5f,
	0x4e, 0x41, 0x4d, 0x45, 0x5f, 0x43, 0x4c, 0x55, 0x53, 0x54, 0x45, 0x52, 0x5f, 0x47, 0x4c, 0x4f,
	0x42, 0x41, 0x4c, 0x5f, 0x4d, 0x4f, 0x55, 0x4e, 0x54, 0x5f, 0x44, 0x45, 0x4c, 0x45, 0x54, 0x45,
	0x44, 0x10, 0x18, 0x12, 0x2e, 0x0a, 0x2a, 0x45, 0x56, 0x45, 0x4e, 0x54, 0x5f, 0x4e, 0x41, 0x4d,
	0x45, 0x5f, 0x43, 0x4c, 0x55, 0x53, 0x54, 0x45, 0x52, 0x5f, 0x50, 0x41, 0x52, 0x54, 0x49, 0x54,
	0x49, 0x4f, 0x4e, 0x5f, 0x4d, 0x4f, 0x55, 0x4e, 0x54, 0x5f, 0x43, 0x52, 0x45, 0x41, 0x54, 0x45,
	0x44, 0x10, 0x16, 0x12, 0x2e, 0x0a, 0x2a, 0x45, 0x56, 0x45, 0x4e, 0x54, 0x5f, 0x4e, 0x41, 0x4d,
	0x45, 0x5f, 0x43, 0x4c, 0x55, 0x53, 0x54, 0x45, 0x52, 0x5f, 0x50, 0x41, 0x52, 0x54, 0x49, 0x54,
	0x49, 0x4f, 0x4e, 0x5f, 0x4d, 0x4f, 0x55, 0x4e, 0x54, 0x5f, 0x44, 0x45, 0x4c, 0x45, 0x54, 0x45,
	0x44, 0x10, 0x19, 0x12, 0x25, 0x0a, 0x21, 0x45, 0x56, 0x45, 0x4e, 0x54, 0x5f, 0x4e, 0x41, 0x4d,
	0x45, 0x5f, 0x43, 0x4c, 0x55, 0x53, 0x54, 0x45, 0x52, 0x5f, 0x4d, 0x4f, 0x55, 0x4e, 0x54, 0x5f,
	0x54, 0x52, 0x41, 0x4e, 0x53, 0x46, 0x45, 0x52, 0x10, 0x1a, 0x12, 0x32, 0x0a, 0x2e, 0x45, 0x56,
	0x45, 0x4e, 0x54, 0x5f, 0x4e, 0x41, 0x4d, 0x45, 0x5f, 0x43, 0x4c, 0x55, 0x53, 0x54, 0x45, 0x52,
	0x5f, 0x53, 0x45, 0x43, 0x55, 0x52, 0x49, 0x54, 0x59, 0x5f, 0x47, 0x52, 0x4f, 0x55, 0x50, 0x5f,
	0x52, 0x55, 0x4c, 0x45, 0x5f, 0x55, 0x50, 0x44, 0x41, 0x54, 0x45, 0x44, 0x10, 0x1b, 0x12, 0x2d,
	0x0a, 0x29, 0x45, 0x56, 0x45, 0x4e, 0x54, 0x5f, 0x4e, 0x41, 0x4d, 0x45, 0x5f, 0x43, 0x4c, 0x55,
	0x53, 0x54, 0x45, 0x52, 0x5f, 0x41, 0x55, 0x54, 0x4f, 0x53, 0x43, 0x41, 0x4c, 0x45, 0x5f, 0x4e,
	0x4f, 0x44, 0x45, 0x5f, 0x43, 0x52, 0x45, 0x41, 0x54, 0x45, 0x44, 0x10, 0x1c, 0x12, 0x30, 0x0a,
	0x2c, 0x45, 0x56, 0x45, 0x4e, 0x54, 0x5f, 0x4e, 0x41, 0x4d, 0x45, 0x5f, 0x43, 0x4c, 0x55, 0x53,
	0x54, 0x45, 0x52, 0x5f, 0x41, 0x55, 0x54, 0x4f, 0x53, 0x43, 0x41, 0x4c, 0x45, 0x5f, 0x4e, 0x4f,
	0x44, 0x45, 0x5f, 0x54, 0x45, 0x52, 0x4d, 0x49, 0x4e, 0x41, 0x54, 0x45, 0x44, 0x10, 0x1d, 0x12,
	0x24, 0x0a, 0x20, 0x45, 0x56, 0x45, 0x4e, 0x54, 0x5f, 0x4e, 0x41, 0x4d, 0x45, 0x5f, 0x53, 0x54,
	0x4f, 0x52, 0x41, 0x47, 0x45, 0x5f, 0x46, 0x49, 0x4c, 0x45, 0x5f, 0x55, 0x50, 0x4c, 0x4f, 0x41,
	0x44, 0x45, 0x44, 0x10, 0x1e, 0x12, 0x26, 0x0a, 0x22, 0x45, 0x56, 0x45, 0x4e, 0x54, 0x5f, 0x4e,
	0x41, 0x4d, 0x45, 0x5f, 0x53, 0x54, 0x4f, 0x52, 0x41, 0x47, 0x45, 0x5f, 0x46, 0x49, 0x4c, 0x45,
	0x5f, 0x44, 0x4f, 0x57, 0x4e, 0x4c, 0x4f, 0x41, 0x44, 0x45, 0x44, 0x10, 0x1f, 0x12, 0x21, 0x0a,
	0x1d, 0x45, 0x56, 0x45, 0x4e, 0x54, 0x5f, 0x4e, 0x41, 0x4d, 0x45, 0x5f, 0x53, 0x54, 0x4f, 0x52,
	0x41, 0x47, 0x45, 0x5f, 0x46, 0x49, 0x4c, 0x45, 0x5f, 0x4d, 0x4f, 0x56, 0x45, 0x44, 0x10, 0x20,
	0x12, 0x22, 0x0a, 0x1e, 0x45, 0x56, 0x45, 0x4e, 0x54, 0x5f, 0x4e, 0x41, 0x4d, 0x45, 0x5f, 0x53,
	0x54, 0x4f, 0x52, 0x41, 0x47, 0x45, 0x5f, 0x46, 0x49, 0x4c, 0x45, 0x5f, 0x43, 0x4f, 0x50, 0x49,
	0x45, 0x44, 0x10, 0x21, 0x12, 0x23, 0x0a, 0x1f, 0x45, 0x56, 0x45, 0x4e, 0x54, 0x5f, 0x4e, 0x41,
	0x4d, 0x45, 0x5f, 0x53, 0x54, 0x4f, 0x52, 0x41, 0x47, 0x45, 0x5f, 0x46, 0x49, 0x4c, 0x45, 0x5f,
	0x52, 0x45, 0x4e, 0x41, 0x4d, 0x45, 0x44, 0x10, 0x22, 0x12, 0x23, 0x0a, 0x1f, 0x45, 0x56, 0x45,
	0x4e, 0x54, 0x5f, 0x4e, 0x41, 0x4d, 0x45, 0x5f, 0x53, 0x54, 0x4f, 0x52, 0x41, 0x47, 0x45, 0x5f,
	0x46, 0x49, 0x4c, 0x45, 0x5f, 0x44, 0x45, 0x4c, 0x45, 0x54, 0x45, 0x44, 0x10, 0x23, 0x12, 0x24,
	0x0a, 0x20, 0x45, 0x56, 0x45, 0x4e, 0x54, 0x5f, 0x4e, 0x41, 0x4d, 0x45, 0x5f, 0x53, 0x54, 0x4f,
	0x52, 0x41, 0x47, 0x45, 0x5f, 0x46, 0x49, 0x4c, 0x45, 0x5f, 0x50, 0x52, 0x45, 0x56, 0x49, 0x45,
	0x57, 0x44, 0x10, 0x24, 0x12, 0x23, 0x0a, 0x1f, 0x45, 0x56, 0x45, 0x4e, 0x54, 0x5f, 0x4e, 0x41,
	0x4d, 0x45, 0x5f, 0x53, 0x54, 0x4f, 0x52, 0x41, 0x47, 0x45, 0x5f, 0x46, 0x49, 0x4c, 0x45, 0x5f,
	0x43, 0x52, 0x45, 0x41, 0x54, 0x45, 0x44, 0x10, 0x25, 0x2a, 0x57, 0x0a, 0x0a, 0x45, 0x6e, 0x74,
	0x69, 0x74, 0x79, 0x54, 0x79, 0x70, 0x65, 0x12, 0x17, 0x0a, 0x13, 0x45, 0x4e, 0x54, 0x49, 0x54,
	0x59, 0x5f, 0x54, 0x59, 0x50, 0x45, 0x5f, 0x49, 0x4e, 0x56, 0x41, 0x4c, 0x49, 0x44, 0x10, 0x00,
	0x12, 0x17, 0x0a, 0x13, 0x45, 0x4e, 0x54, 0x49, 0x54, 0x59, 0x5f, 0x54, 0x59, 0x50, 0x45, 0x5f,
	0x43, 0x4c, 0x55, 0x53, 0x54, 0x45, 0x52, 0x10, 0x01, 0x12, 0x17, 0x0a, 0x13, 0x45, 0x4e, 0x54,
	0x49, 0x54, 0x59, 0x5f, 0x54, 0x59, 0x50, 0x45, 0x5f, 0x53, 0x54, 0x4f, 0x52, 0x41, 0x47, 0x45,
	0x10, 0x02, 0x42, 0xa8, 0x01, 0x0a, 0x0f, 0x63, 0x6f, 0x6d, 0x2e, 0x61, 0x75, 0x64, 0x69, 0x74,
	0x69, 0x6e, 0x67, 0x2e, 0x76, 0x31, 0x42, 0x09, 0x45, 0x6e, 0x75, 0x6d, 0x50, 0x72, 0x6f, 0x74,
	0x6f, 0x50, 0x01, 0x5a, 0x3d, 0x67, 0x69, 0x74, 0x2e, 0x66, 0x61, 0x73, 0x74, 0x6f, 0x6e, 0x65,
	0x74, 0x65, 0x63, 0x68, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x66, 0x61, 0x73, 0x74, 0x6f, 0x6e, 0x65,
	0x2f, 0x61, 0x70, 0x69, 0x2d, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x61, 0x63, 0x74, 0x2f, 0x67, 0x65,
	0x6e, 0x2f, 0x67, 0x6f, 0x2f, 0x76, 0x31, 0x3b, 0x61, 0x75, 0x64, 0x69, 0x74, 0x69, 0x6e, 0x67,
	0x76, 0x31, 0xa2, 0x02, 0x03, 0x41, 0x58, 0x58, 0xaa, 0x02, 0x0b, 0x41, 0x75, 0x64, 0x69, 0x74,
	0x69, 0x6e, 0x67, 0x2e, 0x56, 0x31, 0xca, 0x02, 0x0b, 0x41, 0x75, 0x64, 0x69, 0x74, 0x69, 0x6e,
	0x67, 0x5c, 0x56, 0x31, 0xe2, 0x02, 0x17, 0x41, 0x75, 0x64, 0x69, 0x74, 0x69, 0x6e, 0x67, 0x5c,
	0x56, 0x31, 0x5c, 0x47, 0x50, 0x42, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0xea, 0x02,
	0x0c, 0x41, 0x75, 0x64, 0x69, 0x74, 0x69, 0x6e, 0x67, 0x3a, 0x3a, 0x56, 0x31, 0x62, 0x06, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_v1_enum_proto_rawDescOnce sync.Once
	file_v1_enum_proto_rawDescData = file_v1_enum_proto_rawDesc
)

func file_v1_enum_proto_rawDescGZIP() []byte {
	file_v1_enum_proto_rawDescOnce.Do(func() {
		file_v1_enum_proto_rawDescData = protoimpl.X.CompressGZIP(file_v1_enum_proto_rawDescData)
	})
	return file_v1_enum_proto_rawDescData
}

var file_v1_enum_proto_enumTypes = make([]protoimpl.EnumInfo, 2)
var file_v1_enum_proto_goTypes = []interface{}{
	(EventName)(0),  // 0: auditing.v1.EventName
	(EntityType)(0), // 1: auditing.v1.EntityType
}
var file_v1_enum_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_v1_enum_proto_init() }
func file_v1_enum_proto_init() {
	if File_v1_enum_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_v1_enum_proto_rawDesc,
			NumEnums:      2,
			NumMessages:   0,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_v1_enum_proto_goTypes,
		DependencyIndexes: file_v1_enum_proto_depIdxs,
		EnumInfos:         file_v1_enum_proto_enumTypes,
	}.Build()
	File_v1_enum_proto = out.File
	file_v1_enum_proto_rawDesc = nil
	file_v1_enum_proto_goTypes = nil
	file_v1_enum_proto_depIdxs = nil
}
