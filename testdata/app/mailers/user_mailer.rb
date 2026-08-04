class UserMailer < ApplicationMailer
  default from: "noreply@example.com", reply_to: "support@example.com"
  layout "mailer"
  include HeaderFooter

  def welcome_email
    attachments["invoice.pdf"] = File.read("/tmp/invoice.pdf")
    mail(to: "user@example.com", subject: "Welcome")
  end

  def shipment_notification
    attachments.inline["logo.png"] = File.binread("/tmp/logo.png")
    mail(to: "user@example.com", subject: "Your shipment")
  end

  private def internal_helper
    attachments["secret.txt"] = "data"
  end
end