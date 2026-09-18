package app

import (
	"context"
	"testing"

	"eiksy/internal/domain/workspace"
	"eiksy/internal/storage/memory"
)

type lockTrackingSSHManager struct{ disconnected []string }
func (m *lockTrackingSSHManager) Connect(context.Context,string,string,int,string,string,map[string]string) error{return nil}
func (m *lockTrackingSSHManager) SendInput(string,string) error{return nil}
func (m *lockTrackingSSHManager) ResizeTerminal(string,int,int) error{return nil}
func (m *lockTrackingSSHManager) Disconnect(id string) error{m.disconnected=append(m.disconnected,id);return nil}
func (m *lockTrackingSSHManager) SetOutputHandler(string,func(string)){}
func (m *lockTrackingSSHManager) GetCurrentDir(string)(string,error){return ".",nil}
func (m *lockTrackingSSHManager) AcceptHostKey(string) error{return nil}

func TestLockSecureStorageDisconnectsRuntimeSSH(t *testing.T) {
	store:=memory.NewStore()
	store.OpenRuntimeTab(workspace.Tab{ID:"ssh-1",Title:"one",ProtocolID:"ssh"})
	store.OpenRuntimeTab(workspace.Tab{ID:"ssh-2",Title:"two",ProtocolID:"ssh"})
	manager:=&lockTrackingSSHManager{}
	service:=NewService(store,manager,nil)
	service.LockSecureStorage()
	if len(manager.disconnected)!=2 {t.Fatalf("expected 2 SSH disconnects, got %d",len(manager.disconnected))}
	if store.SecureStorageStatus().Unlocked {t.Fatal("expected secure storage to be locked")}
}
