require "json"
require "prism"

RAILS_MACROS = %i[
  belongs_to has_and_belongs_to_many has_many has_one
  validates validate validates_absence_of validates_acceptance_of
  validates_confirmation_of validates_exclusion_of validates_format_of
  validates_inclusion_of validates_length_of validates_numericality_of
  validates_presence_of validates_size_of validates_uniqueness_of
  scope before_validation after_validation before_create after_create after_create_commit
  before_update after_update after_update_commit before_save after_save after_save_commit before_destroy after_destroy after_destroy_commit
  after_initialize before_commit after_commit after_rollback after_touch
  after_find around_validation around_create around_update around_save
  around_destroy enum delegate
].freeze

VISIBILITY_CALLS = %i[public protected private].freeze

def line_range(node)
  {
    "start_line" => node.location.start_line,
    "end_line" => node.location.end_line
  }
end

def name_for(node)
  return nil unless node
  return node.slice if node.respond_to?(:slice)

  nil
end

def arg_slices(call)
  return [] unless call.arguments

  call.arguments.arguments.map(&:slice)
end

def call_entry(node, kind)
  {
    "name" => node.name.to_s,
    "kind" => kind,
    "args" => arg_slices(node),
    "source" => call_source(node),
    "start_line" => node.location.start_line,
    "end_line" => node.location.end_line
  }
end

def call_source(node)
  source = node.slice
  return source unless node.respond_to?(:block) && node.block && source.include?("\n")

  source.lines.first.chomp
end

def constant_entry(node)
  {
    "name" => node.name.to_s,
    "source" => node.slice,
    "start_line" => node.location.start_line,
    "end_line" => node.location.end_line
  }
end

def method_entry(node, visibility)
  {
    "name" => node.name.to_s,
    "params" => node.parameters&.slice.to_s,
    "visibility" => visibility.to_s,
    "start_line" => node.location.start_line,
    "end_line" => node.location.end_line
  }
end

def empty_container
  {
    "classes" => [],
    "modules" => [],
    "constants" => [],
    "calls" => [],
    "methods" => [],
    "includes" => [],
    "extends" => [],
    "prepends" => []
  }
end

def class_entry(node)
  empty_container.merge(
    "name" => name_for(node.constant_path),
    "parent" => name_for(node.superclass),
    "includes" => [],
    "extends" => [],
    "prepends" => [],
    **line_range(node)
  )
end

def module_entry(node)
  empty_container.merge(
    "name" => name_for(node.constant_path),
    "includes" => [],
    "extends" => [],
    "prepends" => [],
    **line_range(node)
  )
end

def body_nodes(node)
  body = if node.respond_to?(:statements)
           node.statements
         elsif node.respond_to?(:body)
           node.body
         end
  return [] unless body && body.respond_to?(:body) && body.body

  body.body
end

def walk_body(nodes, container)
  visibility = :public

  nodes.each do |node|
    case node
    when Prism::ClassNode
      child = class_entry(node)
      walk_body(body_nodes(node), child)
      container["classes"] << child
    when Prism::ModuleNode
      child = module_entry(node)
      walk_body(body_nodes(node), child)
      container["modules"] << child
    when Prism::ConstantWriteNode
      container["constants"] << constant_entry(node)
    when Prism::DefNode
      container["methods"] << method_entry(node, visibility)
    when Prism::CallNode
      if VISIBILITY_CALLS.include?(node.name) && node.arguments.nil?
        visibility = node.name
      elsif node.name == :include
        container["includes"] << call_entry(node, "include")
      elsif node.name == :extend
        container["extends"] << call_entry(node, "extend")
      elsif node.name == :prepend
        container["prepends"] << call_entry(node, "prepend")
      elsif RAILS_MACROS.include?(node.name)
        container["calls"] << call_entry(node, "rails_macro")
      elsif node.receiver.nil? && !node.name.to_s.end_with?("=")
        container["calls"] << call_entry(node, "macro")
      end
    end
  end
end

input = JSON.parse($stdin.read)
paths = input.fetch("paths")

files = paths.map do |path|
  result = Prism.parse_file(path)
  file = empty_container.merge(
    "path" => path,
    "parse_errors" => result.errors.map(&:message)
  )
  walk_body(body_nodes(result.value), file) if result.value
  file
end

puts JSON.generate("files" => files)
