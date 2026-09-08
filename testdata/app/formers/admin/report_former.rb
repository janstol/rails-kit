module Admin
  class ReportFormer
    include ActiveModel::Model

    attr_accessor :start_date, :end_date

    validates :start_date, presence: true

    def generate
      Report.new(start_date: start_date, end_date: end_date)
    end
  end
end
