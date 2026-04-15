package syncengine

import "command-task/internal/cache"

// ResolveTaskConflict applies an MVP last-write-wins strategy.
func ResolveTaskConflict(local cache.TaskEntity, remote cache.TaskEntity) cache.TaskEntity {
	if remote.UpdatedAtUnix >= local.UpdatedAtUnix {
		return remote
	}
	return local
}
