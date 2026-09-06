module UsersHelper
  DEFAULT_AVATAR_SIZE = 40

  include IconHelper

  def user_avatar(user)
    image_tag(user.avatar_url)
  end

  def user_badge(user, size = DEFAULT_AVATAR_SIZE)
    "#{user.name} (#{size})"
  end

  def user_status_tag(user:, css_class: "status")
    content_tag(:span, user.status, class: css_class)
  end

  def user_tags(*tags)
    tags.join(", ")
  end

  def with_user_context(&block)
    capture(&block)
  end

  def current_user_name
    current_user.name
  end

  def user_summary(
    user,
    show_email: false,
    show_role: false
  )
    parts = [user.name]
    parts << user.email if show_email
    parts << user.role if show_role
    parts.join(" / ")
  end

  def self.default_avatar_url
    "/images/default_avatar.png"
  end

  private

  def user_secret_token(user)
    user.secret_token
  end

  private def formatted_user_id(user)
    "USR-#{user.id}"
  end
end
