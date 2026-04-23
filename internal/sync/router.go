package syncengine

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"lazy-click/internal/provider"
	"lazy-click/internal/provider/clickup"
)

type ProviderNode struct {
	meta   ProviderMeta
	engine *Engine
	api    provider.ProjectProvider
}

type Router struct {
	nodes    map[string]ProviderNode
	activeID string
}

func NewRouter(nodes []ProviderNode, activeID string) *Router {
	indexed := make(map[string]ProviderNode, len(nodes))
	for _, node := range nodes {
		if strings.TrimSpace(node.meta.ID) == "" || node.engine == nil || node.api == nil {
			continue
		}
		indexed[node.meta.ID] = node
	}
	if activeID == "" {
		for id := range indexed {
			activeID = id
			break
		}
	}
	if _, ok := indexed[activeID]; !ok {
		activeID = ""
	}
	return &Router{nodes: indexed, activeID: activeID}
}

func (r *Router) Providers() []ProviderMeta {
	out := make([]ProviderMeta, 0, len(r.nodes))
	for _, node := range r.nodes {
		out = append(out, node.meta)
	}
	sort.Slice(out, func(i int, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (r *Router) ActiveProviderID() string {
	return r.activeID
}

func (r *Router) SetActiveProvider(providerID string) bool {
	if providerID == "" {
		return false
	}
	if _, ok := r.nodes[providerID]; !ok {
		return false
	}
	r.activeID = providerID
	return true
}

func (r *Router) activeNode() (ProviderNode, error) {
	node, ok := r.nodes[r.activeID]
	if !ok {
		return ProviderNode{}, fmt.Errorf("active provider unavailable")
	}
	return node, nil
}

func (r *Router) QueueTaskUpdate(taskID string, update provider.TaskUpdate) error {
	node, err := r.activeNode()
	if err != nil {
		return err
	}
	return node.engine.QueueTaskUpdate(taskID, update)
}

func (r *Router) GetCurrentUser(ctx context.Context) (provider.User, error) {
	node, err := r.activeNode()
	if err != nil {
		return provider.User{}, err
	}
	return node.engine.GetCurrentUser(ctx)
}

func (r *Router) QueueAddComment(taskID string, text string, localCommentID string) error {
	node, err := r.activeNode()
	if err != nil {
		return err
	}
	return node.engine.QueueAddComment(taskID, text, localCommentID)
}

func (r *Router) Cycle(ctx context.Context) error {
	node, err := r.activeNode()
	if err != nil {
		return err
	}
	return node.engine.Cycle(ctx)
}

func (r *Router) SyncList(ctx context.Context, listID string) error {
	node, err := r.activeNode()
	if err != nil {
		return err
	}
	return node.engine.SyncList(ctx, listID)
}

func (r *Router) SetActiveListID(listID string) {
	node, err := r.activeNode()
	if err != nil {
		return
	}
	node.engine.SetActiveListID(listID)
}

func (r *Router) RevalidateTask(ctx context.Context, taskID string) error {
	node, err := r.activeNode()
	if err != nil {
		return err
	}
	return node.engine.RevalidateTask(ctx, taskID)
}

func (r *Router) SyncStatus() string {
	node, err := r.activeNode()
	if err != nil {
		return "idle"
	}
	return node.engine.SyncStatus()
}

func (r *Router) ProviderDisplayName() string {
	node, err := r.activeNode()
	if err != nil {
		return "none"
	}
	if strings.TrimSpace(node.meta.DisplayName) == "" {
		return node.meta.ID
	}
	return node.meta.DisplayName
}

func (r *Router) ActiveProvider() provider.ProjectProvider {
	node, err := r.activeNode()
	if err != nil {
		return nil
	}
	return node.api
}

func (r *Router) SetProviderToken(providerID string, token string) bool {
	node, ok := r.nodes[providerID]
	if !ok {
		return false
	}
	if node.meta.Kind != "clickup" {
		return false
	}
	api := clickup.NewFromToken(token)
	node.api = api
	node.engine.SetProviderAPI(api)
	r.nodes[providerID] = node
	return true
}

func BuildProviderNode(meta ProviderMeta, engine *Engine, api provider.ProjectProvider) ProviderNode {
	return ProviderNode{meta: meta, engine: engine, api: api}
}
