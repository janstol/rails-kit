class UserPresenter < BasePresenter
  DEFAULT_AVATAR = "avatar.png"

  attr_reader :user, :view_context, :current_admin

  delegate :to_model, to: :user

  include Rails.application.routes.url_helpers

  def full_name
    "#{user.first_name} #{user.last_name}"
  end

  def formatted_created_at(format: :short)
    user.created_at.strftime(format == :short ? "%m/%d" : "%B %d, %Y")
  end

  def self.build_default
    new(User.new)
  end

  private

  def internal_token
    user.token
  end

  private def secret_hash
    Digest::SHA1.hexdigest(user.token)
  end
end
