package entity

import "mayfly-go/pkg/model"

// OctopIdentityLink Casdoor 稳定 ID 与某 Octop 实例内用户 ID 的人工确认关联。
//
// 显示名/邮箱只可作为候选提示，不得作为唯一键，也不自动合并。
type OctopIdentityLink struct {
	model.Model

	ConnectionId        uint64 `json:"connectionId" gorm:"not null;uniqueIndex:uk_octop_identity_casdoor"`
	CasdoorUserSourceId string `json:"casdoorUserSourceId" gorm:"size:128;not null;uniqueIndex:uk_octop_identity_casdoor"`
	OctopInstanceId     uint64 `json:"octopInstanceId" gorm:"not null;uniqueIndex:uk_octop_identity_casdoor;uniqueIndex:uk_octop_identity_octop"`
	OctopUserId         string `json:"octopUserId" gorm:"size:128;not null;uniqueIndex:uk_octop_identity_octop"`
	Evidence            string `json:"evidence" gorm:"size:255"` // 人工确认依据（不含密钥/正文）
}

func (o *OctopIdentityLink) TableName() string {
	return "t_octop_identity_link"
}
