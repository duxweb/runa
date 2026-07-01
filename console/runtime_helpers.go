package console

import (
	"context"

	"github.com/duxweb/runa/asset"
	"github.com/duxweb/runa/auth"
	"github.com/duxweb/runa/cache"
	"github.com/duxweb/runa/database"
	"github.com/duxweb/runa/event"
	"github.com/duxweb/runa/lock"
	runlog "github.com/duxweb/runa/log"
	"github.com/duxweb/runa/message"
	runaprovider "github.com/duxweb/runa/provider"
	"github.com/duxweb/runa/queue"
	"github.com/duxweb/runa/rate"
	"github.com/duxweb/runa/schedule"
	"github.com/duxweb/runa/session"
	"github.com/duxweb/runa/storage"
	"github.com/duxweb/runa/task"
	"github.com/duxweb/runa/view"
)

func queueRegistry(app AppContext) *queue.Registry {
	registry, _ := runaprovider.Invoke[*queue.Registry](app)
	return registry
}

func queueInfos(ctx context.Context, app AppContext) []queue.QueueInfo {
	if registry := queueRegistry(app); registry != nil {
		return registry.QueueInfo(ctx)
	}
	return nil
}

func workerInfos(ctx context.Context, app AppContext) []queue.WorkerInfo {
	if registry := queueRegistry(app); registry != nil {
		return registry.WorkerInfo(ctx)
	}
	return nil
}

func jobInfos(app AppContext) []queue.JobInfo {
	if registry := queueRegistry(app); registry != nil {
		return registry.JobInfo()
	}
	return nil
}

func messageRegistry(app AppContext) *message.Registry {
	registry, _ := runaprovider.Invoke[*message.Registry](app)
	return registry
}

func messageInfos(app AppContext) []message.BrokerInfo {
	if registry := messageRegistry(app); registry != nil {
		return registry.Info()
	}
	return nil
}

func messageSubscriptionInfos(app AppContext) []message.SubscriptionInfo {
	if registry := messageRegistry(app); registry != nil {
		return registry.SubscriptionInfo()
	}
	return nil
}

func scheduleRegistry(app AppContext) *schedule.Registry {
	registry, _ := runaprovider.Invoke[*schedule.Registry](app)
	return registry
}

func scheduleInfos(app AppContext) []schedule.Info {
	if registry := scheduleRegistry(app); registry != nil {
		return registry.Info()
	}
	return nil
}

func taskInfos(app AppContext) []task.Info {
	registry, _ := runaprovider.Invoke[*task.Registry](app)
	if registry != nil {
		return registry.Info()
	}
	return nil
}

func eventInfos(app AppContext) []event.Info {
	registry, _ := runaprovider.Invoke[*event.Registry](app)
	if registry != nil {
		return registry.Info()
	}
	return nil
}

func databaseInfos(app AppContext) []database.Info {
	registry, _ := runaprovider.Invoke[*database.Registry](app)
	if registry != nil {
		return registry.Info()
	}
	return nil
}

func cacheInfos(app AppContext) []cache.Info {
	registry, _ := runaprovider.Invoke[*cache.Registry](app)
	if registry != nil {
		return registry.Info()
	}
	return nil
}

func storageInfos(app AppContext) []storage.Info {
	registry, _ := runaprovider.Invoke[*storage.Registry](app)
	if registry != nil {
		return registry.Info()
	}
	return nil
}

func sessionInfos(app AppContext) []session.Info {
	registry, _ := runaprovider.Invoke[*session.Registry](app)
	if registry != nil {
		return registry.Info()
	}
	return nil
}

func rateInfos(app AppContext) []rate.Info {
	registry, _ := runaprovider.Invoke[*rate.Registry](app)
	if registry != nil {
		return registry.Info()
	}
	return nil
}

func lockInfos(app AppContext) []lock.Info {
	registry, _ := runaprovider.Invoke[*lock.Registry](app)
	if registry != nil {
		return registry.Info()
	}
	return nil
}

func viewInfos(app AppContext) []view.Info {
	registry, _ := runaprovider.Invoke[*view.Registry](app)
	if registry != nil {
		return registry.Info()
	}
	return nil
}

func assetInfos(app AppContext) []asset.Info {
	registry, _ := runaprovider.Invoke[*asset.Registry](app)
	if registry != nil {
		return registry.Info()
	}
	return nil
}

func logInfos(app AppContext) []runlog.Info {
	registry, _ := runaprovider.Invoke[*runlog.Registry](app)
	if registry != nil {
		return registry.Info()
	}
	return nil
}

func authNames(app AppContext) []string {
	registry, _ := runaprovider.Invoke[*auth.Registry](app)
	if registry != nil {
		return registry.Names()
	}
	return nil
}

func permissionInfos(app AppContext) []auth.PermissionInfo {
	items := []auth.PermissionInfo{}
	if app == nil {
		return items
	}
	for _, item := range appRoutes(app) {
		if item == nil || item.MetaAs[bool]("can") == false && item.MetaData != nil && item.MetaData["can"] != nil {
			continue
		}
		id := item.RouteID()
		if id == "" {
			continue
		}
		items = append(items, auth.PermissionInfo{
			ID:          id,
			Name:        auth.ShortName(id),
			Label:       item.SummaryText,
			Group:       auth.GroupName(id),
			Method:      item.Method,
			Path:        item.Path,
			Tags:        append([]string(nil), item.TagList...),
			Description: item.DescriptionText,
			Meta:        cloneMap(item.MetaData),
		})
	}
	return items
}

func cloneMap(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	cloned := make(map[string]any, len(value))
	for key, item := range value {
		cloned[key] = item
	}
	return cloned
}
