package controller

// Deployment handlers

func (c *CommunityController) handleDeploymentAdd(new interface{}) {
	c.deploymentWorkqueue.Enqueue(new)
}

func (c *CommunityController) handleDeploymentDelete(old interface{}) {
	c.deploymentWorkqueue.Enqueue(old)
}

func (c *CommunityController) handleDeploymentUpdate(old, new interface{}) {
	c.deploymentWorkqueue.Enqueue(new)
}

// Community Configuration handlers

func (c *CommunityController) handleCommunityConfigurationAdd(new interface{}) {
	c.syncCommunityScheduleWorkqueue.Enqueue(new)
}

func (c *CommunityController) handleCommunityConfigurationDelete(old interface{}) {
	c.syncCommunityScheduleWorkqueue.Enqueue(old)
}

func (c *CommunityController) handleCommunityConfigurationUpdate(old, new interface{}) {
	c.syncCommunityScheduleWorkqueue.Enqueue(new)
}
