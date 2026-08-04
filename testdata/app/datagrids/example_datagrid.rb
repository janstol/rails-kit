class ExampleDatagrid < BaseDatagrid
  include Filterable
  include Sortable

  decorate { ExampleDecorator }

  scope do
    Example.active
  end

  filter :name
  filter :latch_model, :enum, select: -> { Example.latch_options }, input_options: { class: 'select2' }
  filter :active, :boolean, default: false do |value, scope|
    scope.where(active: value)
  end

  filter_per_page

  column(:name, &:show_link)
  column :state, order: proc { |direction| direction }, &:state_name
  column(:score, order: "score") { |model| model.score }

  column_actions

  def assets
    Example.all
  end

  def self.build_default
    new
  end

  private def secret_scope
    Example.all
  end

  private

  def base_relation
    Example.all
  end
end