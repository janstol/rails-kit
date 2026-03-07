class Post < ApplicationRecord
  belongs_to :user
  has_many :comments, dependent: :destroy

  validates :title, presence: true, length: { maximum: 255 }
  validates :status, inclusion: { in: %w[draft published archived] }

  scope :published, -> { where(status: "published") }
  scope :recent, -> { order(created_at: :desc) }

  before_save :set_published_at

  enum :status, { draft: "draft", published: "published", archived: "archived" }
end
