module Users
  module Work
    class OverallPresenter
      attr_reader :worker

      def total_hours
        worker.shifts.sum(&:hours)
      end
    end
  end
end
