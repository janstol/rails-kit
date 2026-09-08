class UserExportService
  DEFAULT_LIMIT = 100

  include Searchable

  def initialize(scope = User.all)
    @scope = scope
  end

  def call(format: :csv)
    export(format)
  end

  class << self
    def build(scope: User.all)
      new(scope)
    end
  end

  private

  def export(format)
    format
  end
end
