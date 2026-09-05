package web

import (
	"context"
	"errors"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
	"net/http"
	"sync"
	"time"
)

func (s *server) instanceOperationLock(id string) *sync.RWMutex {
	value, _ := s.instanceOperationLocks.LoadOrStore(id, &sync.RWMutex{})
	return value.(*sync.RWMutex)
}

func (s *server) handleInstanceDelete(w http.ResponseWriter, r *http.Request, id string) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	if id == s.config.DefaultInstanceID {
		writeError(w, http.StatusForbidden, "default_instance_protected", "默认世界不能删除。")
		return
	}
	operationLock := s.instanceOperationLock(id)
	if !operationLock.TryLock() {
		writeError(w, http.StatusConflict, "instance_delete_busy", "世界操作进行中，请稍后重试。")
		return
	}
	defer operationLock.Unlock()
	s.instanceCreateMu.Lock()
	defer s.instanceCreateMu.Unlock()
	// Disconnect does not interrupt a removal half way. A process crash leaves
	// the durable resource plan for a safe explicit retry after restart.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 3*time.Minute)
	defer cancel()
	record, err := s.store.GetInstanceDeletion(ctx, id)
	if err == nil && record.Completed {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		writeError(w, 500, "instance_delete_failed", "无法读取删除状态，请重试。")
		return
	}
	instance, err := s.store.GetInstance(ctx, id)
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, 404, "not_found", "世界不存在。")
		return
	}
	if err != nil {
		writeError(w, 500, "instance_delete_failed", "读取世界失败，请重试。")
		return
	}
	driver, ok := s.loadDriver(w, instance.DriverID)
	if !ok {
		return
	}
	deleter, ok := driver.(interface {
		DeleteInstance(context.Context, registry.Instance, string) error
	})
	if !ok {
		writeError(w, 501, "instance_delete_unavailable", "该游戏尚不支持安全删除。")
		return
	}
	if err = deleter.DeleteInstance(ctx, makeRegistryInstance(instance), s.config.DefaultInstanceID); err != nil {
		var blocked *sj.NewGameOwnerError
		if errors.As(err, &blocked) {
			writeError(w, 409, blocked.Code, blocked.Message)
			return
		}
		writeError(w, 500, "instance_delete_failed", "世界删除未完成，请重试；卡片将保留至清理成功。")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
