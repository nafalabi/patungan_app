package tasks

// DefineTasks registers all available tasks
func DefineTasks() {
	// Register general tasks
	RegisterHandler(TaskLogInfo, LogInfoHandler)

	// Register plan tasks
	RegisterHandler(TaskProcessPlanSchedule, ProcessPlanScheduleHandler)

	// Register notification tasks
	RegisterHandler(TaskSendWhatsappNotif, SendWhatsappNotifHandler)
	RegisterHandler(TaskSendEmailNotif, SendEmailNotifHandler)
}
