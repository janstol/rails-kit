module NotificationService
  DEFAULT_CHANNEL = :email

  include Loggable

  def self.call(notification)
    format(notification)
  end

  def self.format(notification)
    notification
  end
end