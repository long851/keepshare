// Code generated by gorm.io/gen. DO NOT EDIT.
// Code generated by gorm.io/gen. DO NOT EDIT.
// Code generated by gorm.io/gen. DO NOT EDIT.

package model

import (
	"time"
)

const TableNameSharedLink = "keepshare_shared_link"

// SharedLink mapped from table <keepshare_shared_link>
type SharedLink struct {
	AutoID             int64     `gorm:"column:auto_id;primaryKey;autoIncrement:true" json:"auto_id"`
	UserID             string    `gorm:"column:user_id;not null" json:"user_id"`
	State              string    `gorm:"column:state;not null" json:"state"`
	Host               string    `gorm:"column:host;not null" json:"host"`
	CreatedBy          string    `gorm:"column:created_by;not null" json:"created_by"`
	CreatedAt          time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt          time.Time `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP" json:"updated_at"`
	Size               int64     `gorm:"column:size;not null" json:"size"`
	Visitor            int32     `gorm:"column:visitor;not null" json:"visitor"`
	Stored             int32     `gorm:"column:stored;not null" json:"stored"`
	LastVisitedAt      time.Time `gorm:"column:last_visited_at;not null;default:2000-01-01 00:00:00" json:"last_visited_at"`
	LastStoredAt       time.Time `gorm:"column:last_stored_at;not null;default:2000-01-01 00:00:00" json:"last_stored_at"`
	Revenue            int64     `gorm:"column:revenue;not null" json:"revenue"`
	Title              string    `gorm:"column:title;not null" json:"title"`
	OriginalLinkHash   string    `gorm:"column:original_link_hash;not null" json:"original_link_hash"`
	HostSharedLinkHash string    `gorm:"column:host_shared_link_hash;not null" json:"host_shared_link_hash"`
	OriginalLink       string    `gorm:"column:original_link;not null" json:"original_link"`
	HostSharedLink     string    `gorm:"column:host_shared_link;not null" json:"host_shared_link"`
	Error              string    `gorm:"column:error" json:"error"`
}

// TableName SharedLink's table name
func (*SharedLink) TableName() string {
	return TableNameSharedLink
}
