class UserFormer
  include ActiveModel::Model

  DEFAULT_ROLE = "member"

  attr_accessor :name, :email

  validates :name, presence: true
  validates :email, presence: true, format: { with: /@/ }
  validate :email_not_blacklisted

  delegate :to_model, to: :user

  with_options if: -> { role == "admin" } do
    validates :department, presence: true
    validates :approval_code, presence: true
  end

  def save
    return false unless valid?
    user.update(attributes)
  end

  def apply(current_user)
    current_user.update(name: name, email: email)
  end

  def self.build_default
    new(name: "Guest")
  end

  private

  def email_not_blacklisted
    errors.add(:email, "is blacklisted") if Blacklist.include?(email)
  end

  private def user
    @user ||= User.find_or_initialize_by(email: email)
  end
end
