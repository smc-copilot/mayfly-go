package entity

import "mayfly-go/pkg/model"

type InstanceQuery struct {
	model.PageParam
	Name     string `json:"name" form:"name"`
	Center   string `json:"center" form:"center"`
	Status   string `json:"status" form:"status"`
	Keyword  string `json:"keyword" form:"keyword"`
}

type EventQuery struct {
	model.PageParam
	InstanceIds    []uint64 `json:"instanceIds" form:"instanceIds"`
	InstanceId     uint64   `json:"instanceId" form:"instanceId"`
	Category       string   `json:"category" form:"category"`
	Action         string   `json:"action" form:"action"`
	Status         string   `json:"status" form:"status"`
	OctopUserId    string   `json:"octopUserId" form:"octopUserId"`
	CasdoorUserId  string   `json:"casdoorUserId" form:"casdoorUserId"`
	AgentId        string   `json:"agentId" form:"agentId"`
	Keyword        string   `json:"keyword" form:"keyword"`
	StartTime      int64    `json:"startTime" form:"startTime"` // unix ms, UTC 比较
	EndTime        int64    `json:"endTime" form:"endTime"`
}

type CasdoorAppQuery struct {
	model.PageParam
	ConnectionId uint64 `json:"connectionId" form:"connectionId"`
	Organization string `json:"organization" form:"organization"`
	Enabled      *int8  `json:"enabled" form:"enabled"`
	Keyword      string `json:"keyword" form:"keyword"`
}

type CasdoorUserQuery struct {
	model.PageParam
	ConnectionId uint64 `json:"connectionId" form:"connectionId"`
	Organization string `json:"organization" form:"organization"`
	Enabled      *int8  `json:"enabled" form:"enabled"`
	Keyword      string `json:"keyword" form:"keyword"`
}

type IdentityCandidateQuery struct {
	ConnectionId uint64 `json:"connectionId" form:"connectionId"`
	InstanceId   uint64 `json:"instanceId" form:"instanceId"`
	Username     string `json:"username" form:"username"`
	Email        string `json:"email" form:"email"`
}
