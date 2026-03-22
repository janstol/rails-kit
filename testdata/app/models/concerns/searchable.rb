module Searchable
  extend ActiveSupport::Concern

  included do
    scope :search, ->(query) { where("name LIKE ?", "%#{query}%") }
  end

  class_methods do
    def search_all(query)
      search(query).to_a
    end
  end

  def search_highlight
    # highlights search terms in the result
  end

  def search_excerpt
    # returns a short excerpt around the match
  end

  private

  def build_search_query(term)
    # internal helper
    "%#{term}%"
  end
end
