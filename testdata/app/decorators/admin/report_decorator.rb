module Admin
  class ReportDecorator < ApplicationDecorator
    delegate_all

    def summary_line
      "#{object.title} (#{object.status})"
    end
  end
end
