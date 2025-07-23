package messages

const (
	// Команды
	StartMessage = "Привет! Я бот для проверки подписки. Используйте /check для проверки подписки и получения конфигурации VPN."
	HelpMessage  = "Используйте /check для проверки подписки и получения конфигурации VPN."

	// Ошибки
	SubscriptionCheckError = "Произошла ошибка при проверке подписки. Пожалуйста, попробуйте позже."
	ConfigGenerationError  = "Произошла ошибка при генерации конфигурации. Пожалуйста, попробуйте позже."

	// Успешные сообщения
	SubscribedMessage    = "Вы подписаны на канал! \n\nВаш UUID: `%s`\n\nВаша VLESS конфигурация:\n`%s`"
	NotSubscribedMessage = "Вы не подписаны на канал %s. Пожалуйста, подпишитесь и попробуйте снова."

	// Уведомления
	UnsubscriptionNotification = "Вы отписались от канала %s. Ваш доступ к VPN был аннулирован. Чтобы восстановить доступ, подпишитесь на канал и используйте команду /check."
)

// GetSubscribedMessage форматирует сообщение для подписанного пользователя
func GetSubscribedMessage(uuid, vlessURL string) string {
	return SubscribedMessage
}

// GetNotSubscribedMessage форматирует сообщение для неподписанного пользователя
func GetNotSubscribedMessage(channelUsername string) string {
	return NotSubscribedMessage
}

// GetUnsubscriptionNotification форматирует уведомление об отписке
func GetUnsubscriptionNotification(channelUsername string) string {
	return UnsubscriptionNotification
}
