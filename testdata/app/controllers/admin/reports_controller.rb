class Admin::ReportsController < Admin::BaseController
  before_action :require_admin!

  def index
    @reports = Report.all
  end

  def show
    @report = Report.find(params[:id])
  end

  private

  def require_admin!
    redirect_to root_path unless current_user&.admin?
  end
end
