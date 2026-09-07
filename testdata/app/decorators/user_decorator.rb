class UserDecorator < ApplicationDecorator
  DEFAULT_AVATAR = "avatar.png"

  delegate_all

  include Rails.application.routes.url_helpers
  include ActionView::Helpers::NumberHelper

  def full_name
    "#{object.first_name} #{object.last_name}"
  end

  def formatted_created_at(format: :short)
    object.created_at.strftime(format == :short ? "%m/%d" : "%B %d, %Y")
  end

  def self.build_default
    new(User.new)
  end

  private

  def internal_token
    object.token
  end

  private def secret_hash
    Digest::SHA1.hexdigest(object.token)
  end
end
