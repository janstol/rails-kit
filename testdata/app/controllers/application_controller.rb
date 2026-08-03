class ApplicationController < ActionController::Base
  include Authenticatable

  helper_method :current_user, :current_account

  rescue_from ActiveRecord::RecordNotFound, with: :render_not_found

  before_action :set_locale

  private

  def set_locale
    I18n.locale = params[:locale] || I18n.default_locale
  end

  def render_not_found
    render plain: "Not Found", status: :not_found
  end
end
