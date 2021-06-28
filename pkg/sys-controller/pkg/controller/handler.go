package controller

func (c *SystemController) handleCommunitySettingsAdd(new interface{}) {
	c.workqueue.Enqueue(new)
}

func (c *SystemController) handleCommunitySettingsDeletion(old interface{}) {
	c.workqueue.Enqueue(old)
}

func (c *SystemController) handleCommunitySettingsUpdate(old, new interface{}) {
	c.workqueue.Enqueue(new)
}
