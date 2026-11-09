package application

import (
	"context"
	"testing"
	"time"
)

func TestConnectionLockSerializesSameConnectionAndAllowsOthers(t *testing.T) {
	locks := NewConnectionLocks()
	release, err := locks.Acquire(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	other, err := locks.Acquire(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	other()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, err := locks.Acquire(ctx, 1); err == nil {
		t.Fatal("相同连接必须等待持锁操作")
	}
	release()
	again, err := locks.Acquire(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	again()
}

func TestConnectionLockRejectsInvalidID(t *testing.T) {
	if _, err := NewConnectionLocks().Acquire(context.Background(), 0); err == nil {
		t.Fatal("无效连接不能获取锁")
	}
}
