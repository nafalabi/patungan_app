package tasks

// DefineTasks registers all available tasks
func DefineTasks() {
	// Register general tasks
	RegisterHandler(LogInfoTask.TaskID(), LogInfoTask.HandleExecution)

	// Register plan tasks
	RegisterHandler(ProcessPlanScheduleTask.TaskID(), ProcessPlanScheduleTask.HandleExecution)

	// Register notification tasks
	RegisterHandler(SendNotificationTask.TaskID(), SendNotificationTask.HandleExecution)
}
