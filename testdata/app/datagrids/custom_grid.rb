class CustomGrid
  include Exportable

  my_filter :name
  my_column :total
  batch_size 50

  def rows
    []
  end

  def self.default_grid
    new
  end

  private

  def normalized_rows
    rows
  end
end