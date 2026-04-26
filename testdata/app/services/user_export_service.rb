class UserExportService
  DEFAULT_LIMIT = 100

  include Searchable

  def initialize(scope = User.all)
    @scope = scope
  end

  def call(format: :csv)
    export(format)
  end

  private

  def export(format)
    format
  end
end
