module Admin
  module ReportsHelper
    def report_title(report)
      report.title
    end

    def report_period(report)
      "#{report.start_date} - #{report.end_date}"
    end
  end
end
