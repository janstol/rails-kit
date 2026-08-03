class UsersController < ApplicationController
  before_action :authenticate_user!
  before_action :set_user, only: [:show, :edit, :update, :destroy]
  skip_before_action :authenticate_user!, only: [:index], if: :public_action?
  after_action :log_action
  around_action :measure_time

  rescue_from ActiveRecord::RecordNotFound, ActiveRecord::RecordInvalid, with: :handle_not_found
  rescue_from StandardError do |exception|
    render plain: exception.message, status: :internal_server_error
  end

  helper_method :user_display_name

  layout "users"

  respond_to :html, :json

  def index
    @users = User.all
  end

  def show
  end

  def create
    @user = User.new(user_params)
    if @user.save
      redirect_to @user
    else
      render :new
    end
  end

  def update
    @user.update(user_params)
  end

  def destroy
    @user.destroy
  end

  private

  def set_user
    @user = User.find(params[:id])
  end

  private def public_action?
    true
  end

  def user_display_name(user)
    user.name.presence || user.email
  end

  def user_params
    params.require(:user).permit(:name, :email, tags: [])
  end

  def handle_not_found
    render plain: "Not Found", status: :not_found
  end

  def measure_time
    start = Time.now
    yield
    Rails.logger.info(Time.now - start)
  end

  def log_action
    Rails.logger.info(action_name)
  end
end
