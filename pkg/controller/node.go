/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	apiv1 "k8s.io/api/core/v1"
	listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/ingress-gce/pkg/context"
	"k8s.io/ingress-gce/pkg/instances"
	"k8s.io/ingress-gce/pkg/utils"
)

// NodeController synchronizes the state of the nodes to the unmanaged instance
// groups.
type NodeController struct {
	// lister is a cache of the k8s Node resources.
	lister cache.Indexer
	// queue is the TaskQueue used to manage the node worker updates.
	queue utils.TaskQueue
	// instancePool is a NodePool to manage kubernetes nodes.
	instancePool instances.NodePool
}

// NewNodeController returns a new node update controller.
func NewNodeController(ctx *context.ControllerContext, instancePool instances.NodePool) *NodeController {
	c := &NodeController{
		lister:       ctx.NodeInformer.GetIndexer(),
		instancePool: instancePool,
	}
	c.queue = utils.NewPeriodicTaskQueue("", "nodes", c.sync)

	ctx.NodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.queue.Enqueue(obj)
		},
		DeleteFunc: func(obj interface{}) {
			c.queue.Enqueue(obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			if nodeStatusChanged(oldObj.(*apiv1.Node), newObj.(*apiv1.Node)) {
				c.queue.Enqueue(newObj)
			}
		},
	})
	return c
}

// Run a goroutine to process updates for the controller.
func (c *NodeController) Run() {
	c.queue.Run()
}

// Shutdown shuts down the goroutine that processes node updates.
func (c *NodeController) Shutdown() {
	c.queue.Shutdown()
}

func (c *NodeController) sync(key string) error {
	nodeNames, err := utils.GetReadyNodeNames(listers.NewNodeLister(c.lister))
	if err != nil {
		return err
	}
	return c.instancePool.Sync(nodeNames)
}
