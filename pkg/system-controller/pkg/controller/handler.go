package controller

func (c *SystemController) handleCommunityConfigurationsAdd(new interface{}) {
	c.workqueue.Enqueue(new)
}

func (c *SystemController) handleCommunityConfigurationsDeletion(old interface{}) {
	c.workqueue.Enqueue(old)
}

func (c *SystemController) handleCommunityConfigurationsUpdate(old, new interface{}) {
	c.workqueue.Enqueue(new)
}
