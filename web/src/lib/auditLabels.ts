const labels: Record<string, string> = {
  login: 'Вход в панель',
  logout: 'Выход из панели',
  login_failed: 'Неудачный вход',
  totp_enabled: 'Включена 2FA',
  totp_disabled: 'Отключена 2FA',
  recovery_code_used: 'Использован код восстановления',
  password_reset: 'Сброс пароля',
  user_create: 'Создан пользователь',
  user_update: 'Изменён пользователь',
  user_delete: 'Удалён пользователь',
  user_inbound_attach: 'Привязан инбаунд к пользователю',
  user_inbound_detach: 'Отвязан инбаунд от пользователя',
  inbound_create: 'Создан инбаунд',
  inbound_update: 'Изменён инбаунд',
  inbound_delete: 'Удалён инбаунд',
  password_change: 'Пароль изменён',
  sessions_revoked: 'Завершены все сессии, кроме текущей',
  session_revoked: 'Сессия завершена',
}

export function auditLabel(action: string): string {
  return labels[action] ?? action
}
