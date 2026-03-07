class User < ApplicationRecord
  include Searchable
  include Auditable

  has_many :posts, dependent: :destroy
  has_many :comments, dependent: :destroy
  has_many :published_posts, class_name: "Post",
    through: :posts

  validates :email, presence: true, uniqueness: true, format: { with: URI::MailTo::EMAIL_REGEXP }
  validates :name, presence: true, length: { minimum: 2 }
  validate :email_not_banned

  scope :active, -> { where(active: true) }
  scope :admins, -> { where(role: "admin") }
  scope :by_name, ->(name) { where("name ILIKE ?", "%#{name}%") }

  before_validation :normalize_email
  after_create_commit :notify_admin
  after_save :invalidate_cache
  after_commit :sync_to_search_index
  after_touch :update_counter_cache

  enum :role, { member: "member", admin: "admin", moderator: "moderator" }

  delegate :title, :body, to: :latest_post, prefix: true, allow_nil: true
end
