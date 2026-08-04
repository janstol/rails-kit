class Admin::ExportJob < ApplicationJob
  queue_as :low

  def perform
  end
end