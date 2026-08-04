class Admin::NotificationMailer < ApplicationMailer
  default to: "admin@example.com"

  def shipment_notification
    mail(subject: "Admin shipment")
  end
end